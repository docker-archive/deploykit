package loadbalancer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"

	"github.com/Sirupsen/logrus"
)

// BackendSet reference from https://docs.us-phoenix-1.oraclecloud.com/api/#/en/loadbalancer/20170115/BackendSet/
type BackendSet struct {
	Backends           []Backend                        `json:"backends,omitempty"`
	HealthChecker      *HealthChecker                   `json:"healthChecker,omitempty"`
	Name               string                           `json:"name"`
	Policy             string                           `json:"policy"`
	SSLConfig          *SSLConfiguration                `json:"sslConfiguration,omitempty"`
	SessionPersistence *SessionPersistenceConfiguration `json:"sessionPersistenceConfiguration,omitempty"`
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
func (c *Client) CreateBackendSet(loadBalancerID string, backendSet *BackendSet) (bool, *bmc.Error) {
	loadBalancerID = url.PathEscape(loadBalancerID)
	logrus.Info("Set: ", backendSet)
	resp, err := c.Request("POST", fmt.Sprintf("/loadBalancers/%s/backendSets", loadBalancerID), *backendSet)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return false, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 204 {
		return false, bmc.NewError(*resp)
	}
	return true, nil
}

// GetBackendSet gets the health check policy information for a given load balancer and backend set.
func (c *Client) GetBackendSet(loadBalancerID string, backendSetName string) (BackendSet, *bmc.Error) {
	backendSet := BackendSet{}
	loadBalancerID = url.PathEscape(loadBalancerID)
	backendSetName = url.PathEscape(backendSetName)
	resp, err := c.Request("GET", fmt.Sprintf("/loadBalancers/%s/backendSets/%s", loadBalancerID, backendSetName), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return backendSet, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return backendSet, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}

	if err = json.Unmarshal(body, &backendSet); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return backendSet, nil
}

// ListBackendSet gets the health check policy information for a given load balancer and backend set.
func (c *Client) ListBackendSet(loadBalancerID string) ([]BackendSet, *bmc.Error) {
	backendSets := []BackendSet{}
	loadBalancerID = url.QueryEscape(loadBalancerID)
	resp, err := c.Request("GET", fmt.Sprintf("/loadBalancers/%s/backendSets", loadBalancerID), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return backendSets, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return backendSets, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}

	if err = json.Unmarshal(body, &backendSets); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return backendSets, nil
}

// UpdateBackendSet updates a backend set to a load balancer
func (c *Client) UpdateBackendSet(listener *BackendSet) {
	// PUT loadBalancers/{loadBalancerId}/listeners/{listenerName}
	logrus.Warning("Method not yet implemented")
	return
}

// DeleteBackendSet deletes a backend set to a load balancer
func (c *Client) DeleteBackendSet(loadBalancerID string, backendSetName string) (bool, *bmc.Error) {
	loadBalancerID = url.PathEscape(loadBalancerID)
	backendSetName = url.PathEscape(backendSetName)
	resp, err := c.Request("DELETE", fmt.Sprintf("/loadBalancers/%s/backendSets/%s", loadBalancerID, backendSetName), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return false, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 204 {
		return false, bmc.NewError(*resp)
	}
	return true, nil
}
