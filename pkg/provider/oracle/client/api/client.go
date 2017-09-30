package api

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"
	"golang.org/x/crypto/ssh"
)

const (
	// EndpointFormat provides the base template for all API url
	EndpointFormat = "https://iaas.%s.oraclecloud.com/%s/"
	// CoreAPIVersion is the API version for core services
	CoreAPIVersion = "20160918"
	// LoadBalancerAPIVersion is the API version for load balancing services
	LoadBalancerAPIVersion = "20170115"
	metaDataURL            = "http://169.254.169.254/opc/v1/instance/"
)

// Config holds the general API bmcConf
type config struct {
}

// Client represents the struct for basic api calls
type Client struct {
	apiKey        string
	apiPrivateKey *rsa.PrivateKey
	APIRegion     string
	httpClient    *http.Client
}

type metaData struct {
	AvailabilityDomain string `json:"availabilityDomain"`
	CompartmentID      string `json:"compartmentId"`
	DisplayName        string `json:"displayName"`
	ID                 string `json:"id"`
	Image              string `json:"image"`
	Metadata           struct {
		PublicKey string `json:"ssh_authorized_keys"`
		UserData  string `json:"user_data"`
	} `json:"metadata"`
	Region      string `json:"region"`
	Shape       string `json:"shape"`
	State       string `json:"state"`
	TimeCreated string `json:"timeCreated"`
}

// NewClient creates a new, unauthenticated compute Client.
func NewClient(bmcConf *bmc.Config) (*Client, error) {
	apiKey := fmt.Sprintf("%s/%s/%s", *bmcConf.Tenancy, *bmcConf.User, *bmcConf.Fingerprint)
	logrus.Debug("Api Key: ", apiKey)
	privateKey, err := loadKeyFromFile(bmcConf.KeyFile, bmcConf.PassPhrase)
	if err != nil {
		// If we failed to read the file because the key does not exist,
		// just issue a warning and continue.
		logrus.Error("private key error: ", err)
		return nil, err
	}

	return &Client{
		apiKey:        apiKey,
		apiPrivateKey: privateKey,
		APIRegion:     getRegion(bmcConf),
		httpClient: &http.Client{
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				TLSHandshakeTimeout: 120 * time.Second},
		},
	}, nil
}

// only load keys that have no password for now :(
func loadKeyFromFile(pemFile *string, passphrase *string) (*rsa.PrivateKey, error) {
	pemPath, err := filepath.Abs(*pemFile)
	logrus.Debugf("Loading key file from: %s - %s", *pemFile, pemPath)
	if err != nil {
		logrus.Errorf("KeyFile error: %s", err)
		return nil, err
	}
	pemBytes, err := ioutil.ReadFile(pemPath)
	if err != nil {
		return nil, err
	}
	// check if a passphrase was given
	var rawKey interface{}
	if passphrase != nil {
		rawKey, err = ssh.ParseRawPrivateKeyWithPassphrase(pemBytes, []byte(*passphrase))
	} else {
		rawKey, err = ssh.ParseRawPrivateKey(pemBytes)
	}
	if err != nil {
		logrus.Errorf("Cannot parse private key: %s", err)
	}
	rsaKey, ok := rawKey.(*rsa.PrivateKey)
	if !ok {
		logrus.Fatalf("Could not create private key: %v", rawKey)
		return nil, err
	}
	return rsaKey, err
}

func getRegion(bmcConf *bmc.Config) string {
	if bmcConf.Region != nil {
		return *bmcConf.Region
	}
	res, err := http.Get(metaDataURL)
	if err != nil {
		log.Fatalf("Error getting metadata: %s", err)
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	var meta = new(metaData)
	err = json.Unmarshal(body, &meta)
	if err != nil {
		log.Fatalf("Error parsing JSON: %s", err)
	}
	return meta.Region
}
