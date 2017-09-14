package lb

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/Sirupsen/logrus"
)

// Backend reference from https://docs.us-phoenix-1.oraclecloud.com/api/#/en/loadbalancer/20170115/Backend/
type Backend struct {
	Backup    bool   `json:"backup"`
	Drain     bool   `json:"drain"`
	IPAddress string `json:"ipAddress"`
	Name      string `json:"name"`
	Offline   bool   `json:"offline"`
	Port      int    `json:"port"`
	Weight    int    `json:"weight"`
}

// CreateBackend adds a backend set to a load balancer
func (c *Client) CreateBackend(loadBalancerID string, backendSetName string, backend *Backend) bool {
	loadBalancerID = url.PathEscape(loadBalancerID)
	backendSetName = url.PathEscape(backendSetName)
	resp, err := c.Client.Request("POST", fmt.Sprintf("/loadBalancers/%s/backendSets/%s/backends", loadBalancerID, backendSetName), *backend)
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

// GetBackend gets the health check policy information for a given load balancer and backend set.
func (c *Client) GetBackend(loadBalancerID string, backendSetName string, backendName string) Backend {
	backendSet := Backend{}
	loadBalancerID = url.PathEscape(loadBalancerID)
	backendSetName = url.PathEscape(backendSetName)
	backendName = url.PathEscape(backendName)
	resp, err := c.Client.Request("GET", fmt.Sprintf("/loadBalancers/%s/backendSets/%s/backends/%s", loadBalancerID, backendSetName, backendName), nil)
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

// ListBackend gets the health check policy information for a given load balancer and backend set.
func (c *Client) ListBackend(loadBalancerID string, backendSetName string) []Backend {
	backendSets := []Backend{}
	loadBalancerID = url.QueryEscape(loadBalancerID)
	backendSetName = url.PathEscape(backendSetName)
	resp, err := c.Client.Request("GET", fmt.Sprintf("/loadBalancers/%s/backendSets/%s/backends", loadBalancerID, backendSetName), nil)
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

// UpdateBackend updates a backend set to a load balancer
func (c *Client) UpdateBackend(listener *Backend) {
	// PUT loadBalancers/{loadBalancerId}/listeners/{listenerName}
	logrus.Warning("Method not yet implemented")
	return
}

// DeleteBackend deletes a backend set to a load balancer
func (c *Client) DeleteBackend(listener *Backend) {
	// DELETE loadBalancers/{loadBalancerId}/backendSets/{backendSetName}/backends/{backendName}
	logrus.Warning("Method not yet implemented")
	return
}
