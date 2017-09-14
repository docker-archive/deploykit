package lb

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/Sirupsen/logrus"
)

// BackendSet reference from https://docs.us-phoenix-1.oraclecloud.com/api/#/en/loadbalancer/20170115/BackendSet/
type BackendSet struct {
	Backends           string `json:"backends"`
	HealthChecker      string `json:"healthChecker"`
	Name               string `json:"name"`
	Policy             string `json:"policy"`
	SSLConfig          string `json:"sslConfiguration"`
	SessionPersistence string `json:"sessionPersistenceConfiguration"`
}

// SSLConfiguration for the struct within the Listener
type SSLConfiguration struct {
	CertName    string `json:"certificateName"`
	VerifyDepth int    `json:"verifyDepth"`
	VerifyPeer  bool   `json:"verifyPeerCertificate"`
}

// SessionPersistenceConfiguration for the struct within the BackendSet
type SessionPersistenceConfiguration struct {
	CookieName      string `json:"cookieName"`
	DisableFallback bool   `json:"disableFallback"`
}

// CreateBackendSet adds a backend set to a load balancer
func (c *Client) CreateBackendSet(loadBalancerID string, backendSet *BackendSet) bool {
	loadBalancerID = url.PathEscape(loadBalancerID)
	resp, err := c.Client.Request("POST", fmt.Sprintf("/loadBalancers/%s/backendSets", loadBalancerID), *backendSet)
	if err != nil {
		logrus.Error(err)
		return false
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 204 {
		return false
	}
	return true
}

// GetBackendSet gets the health check policy information for a given load balancer and backend set.
func (c *Client) GetBackendSet(loadBalancerID string, backendSetName string) BackendSet {
	backendSet := BackendSet{}
	loadBalancerID = url.PathEscape(loadBalancerID)
	backendSetName = url.PathEscape(backendSetName)
	resp, err := c.Client.Request("GET", fmt.Sprintf("/loadBalancers/%s/backendSets/%s", loadBalancerID, backendSetName), nil)
	if err != nil {
		logrus.Error(err)
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}
	if err = json.Unmarshal(body, &backendSet); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return backendSet
}

// ListBackendSet gets the health check policy information for a given load balancer and backend set.
func (c *Client) ListBackendSet(loadBalancerID string) []BackendSet {
	backendSets := []BackendSet{}
	loadBalancerID = url.QueryEscape(loadBalancerID)
	resp, err := c.Client.Request("GET", fmt.Sprintf("/loadBalancers/%s/backendSets", loadBalancerID), nil)
	if err != nil {
		logrus.Error(err)
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}
	if err = json.Unmarshal(body, &backendSets); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return backendSets
}

// UpdateBackendSet updates a backend set to a load balancer
func (c *Client) UpdateBackendSet(listener *BackendSet) {
	// PUT loadBalancers/{loadBalancerId}/listeners/{listenerName}
	logrus.Warning("Method not yet implemented")
	return
}

// DeleteBackendSet deletes a backend set to a load balancer
func (c *Client) DeleteBackendSet(listener *BackendSet) {
	// DELETE loadBalancers/{loadBalancerId}/listeners/{listenerName}
	logrus.Warning("Method not yet implemented")
	return
}
