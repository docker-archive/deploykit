package bmc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/go-ini/ini"
)

const (
	apiEndpointFormat = "https://iaas.%s.oraclecloud.com/%s/"
	apiVersion        = "20160918"
	metaDataURL       = "http://169.254.169.254/opc/v1/instance/"
)

// A Config provides service configuration for service clients.
type Config struct {
	User        *string
	Fingerprint *string
	KeyFile     *string
	PassPhrase  *string
	Tenancy     *string
	Region      *string
	APIEndpoint *url.URL
	MaxRetries  *int
	// An integer value representing the logging level. The default log level
	// is zero (LogOff), which represents no logging. To enable logging set
	// to a LogLevel Value.
	LogLevel *LogLevelType
	// The logger writer interface to write logging messages to. Defaults to
	// standard out.
	Logger Logger
	// The HTTP client to use when sending requests. Defaults to
	// `http.DefaultClient`.
	HTTPClient *http.Client
}

// NewConfig returns a new Config pointer that can be chained
func NewConfig() *Config {
	return &Config{}
}

// String returns a pointer to the string value passed in.
func String(name string) *string {
	return &name
}

func sharedConfigFilename() string {
	if name := os.Getenv("ORACLE_SHARED_CONFIG_FILE"); len(name) > 0 {
		return name
	}

	return filepath.Join(userHomeDir(), ".oraclebmc", "oraclebmc")
}

func userHomeDir() string {
	homeDir := os.Getenv("HOME") // *nix
	if len(homeDir) == 0 {       // windows
		homeDir = os.Getenv("USERPROFILE")
	}

	return homeDir
}

// FromConfigFile retrieves the configuration from the BMC config
func FromConfigFile(profile string) *Config {
	if len(profile) == 0 {
		profile = "DEFAULT"
	}
	filename := sharedConfigFilename()
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		logrus.Fatal("Config file not found")
	}
	f, err := ini.Load(b)
	if err != nil {
		logrus.Fatalf("Invalid config file: %s", err)
	}
	config := NewConfig()
	err = config.setFromIniFile(profile, f)
	if err != nil {
		logrus.Fatalf("Invalid config file: %s", err)
	}
	return config
}

// setFromFile loads the configuration from the file using
// the profile provided. A sharedConfig pointer type value is used so that
// multiple config file loadings can be chained.
//
// Only loads complete logically grouped values, and will not set fields in cfg
// for incomplete grouped values in the config. Such as credentials. For example
// if a config file only includes aws_access_key_id but no aws_secret_access_key
// the aws_access_key_id will be ignored.
func (c *Config) setFromIniFile(profile string, file *ini.File) error {
	section, err := file.GetSection(profile)
	if err != nil {
		// Fallback to to alternate profile name: profile <name>
		section, err = file.GetSection(fmt.Sprintf("profile %s", profile))
		if err != nil {
			logrus.Errorf("Profile does not exist: %s", err)
			return err
		}
	}

	// Shared Credentials
	user := section.Key("user").String()
	fingerprint := section.Key("fingerprint").String()
	keyFile := section.Key("key_file").String()
	tenancy := section.Key("tenancy").String()
	region := section.Key("region").String()
	if len(user) > 0 && len(fingerprint) > 0 {
		c = &Config{
			User:        String(user),
			Fingerprint: String(fingerprint),
			KeyFile:     String(keyFile),
			Tenancy:     String(tenancy),
			Region:      String(region),
			APIEndpoint: GetAPIEndpoint(region),
		}
	}

	return nil
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

// GetAPIEndpoint gets the endpoint from metadata or using the region passed
func GetAPIEndpoint(region string) *url.URL {
	if region != "" {
		apiEndpoint, err := url.Parse(fmt.Sprintf(apiEndpointFormat, "us-phoenix-1", apiVersion))
		if err != nil {
			log.Fatalf("Error parsing API Endpoint: %s", err)
		}
		return apiEndpoint
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
	apiEndpoint, err := url.Parse(fmt.Sprintf(apiEndpointFormat, meta.Region, apiVersion))
	if err != nil {
		log.Fatalf("Error parsing API Endpoint: %s", err)
	}

	return apiEndpoint
}
