package api

import (
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/FrenchBen/oracle-sdk-go/bmc"
	"github.com/Sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// Client represents the struct for basic api calls
type Client struct {
	apiEndpoint   *url.URL
	apiKey        string
	apiPrivateKey *rsa.PrivateKey
	httpClient    *http.Client
}

// NewClient creates a new, unauthenticated compute Client.
func NewClient(config *bmc.Config) (*Client, error) {
	apiKey := fmt.Sprintf("%s/%s/%s", *config.Tenancy, *config.User, *config.Fingerprint)
	logrus.Debug("Api Key: ", apiKey)
	privateKey, err := loadKeyFromFile(config.KeyFile, config.PassPhrase)
	if err != nil {
		// If we failed to read the file because the key does not exist,
		// just issue a warning and continue.
		logrus.Error("private key error: ", err)
		return nil, err
	}

	return &Client{
		apiEndpoint:   config.APIEndpoint,
		apiKey:        apiKey,
		apiPrivateKey: privateKey,
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
