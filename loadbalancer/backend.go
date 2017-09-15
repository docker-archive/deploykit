package loadbalancer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/FrenchBen/oracle-sdk-go/bmc"

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
func (c *Client) CreateBackend(loadBalancerID string, backendSetName string, backend *Backend) (bool, *bmc.Error) {
	loadBalancerID = url.PathEscape(loadBalancerID)
	backendSetName = url.PathEscape(backendSetName)
	resp, err := c.Client.Request("POST", fmt.Sprintf("/loadBalancers/%s/backendSets/%s/backends", loadBalancerID, backendSetName), *backend)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return false, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 204 {
		return false, bmc.NewError(resp)
	}
	return true, nil
}

// GetBackend gets the health check policy information for a given load balancer and backend set.
func (c *Client) GetBackend(loadBalancerID string, backendSetName string, backendName string) (Backend, *bmc.Error) {
	backend := Backend{}
	loadBalancerID = url.PathEscape(loadBalancerID)
	backendSetName = url.PathEscape(backendSetName)
	backendName = url.PathEscape(backendName)
	resp, err := c.Client.Request("GET", fmt.Sprintf("/loadBalancers/%s/backendSets/%s/backends/%s", loadBalancerID, backendSetName, backendName), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return backend, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}
	if resp.StatusCode != 200 {
		return backend, bmc.NewError(resp)
	}
	if err = json.Unmarshal(body, &backend); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return backend, nil
}

// ListBackend gets the health check policy information for a given load balancer and backend set.
func (c *Client) ListBackend(loadBalancerID string, backendSetName string) ([]Backend, *bmc.Error) {
	backends := []Backend{}
	loadBalancerID = url.QueryEscape(loadBalancerID)
	backendSetName = url.PathEscape(backendSetName)
	resp, err := c.Client.Request("GET", fmt.Sprintf("/loadBalancers/%s/backendSets/%s/backends", loadBalancerID, backendSetName), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return backends, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}
	if resp.StatusCode != 200 {
		return backends, bmc.NewError(resp)
	}
	if err = json.Unmarshal(body, &backends); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return backends, nil
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
