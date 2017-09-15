package lb

import (
	"fmt"
	"net/url"

	"github.com/Sirupsen/logrus"
)

// Listener represents the listener's configuration: https://docs.us-phoenix-1.oraclecloud.com/api/#/en/loadbalancer/20170115/Listener/
type Listener struct {
	BackendSetName string `json:"defaultBackendSetName"`
	Name           string `json:"name"`
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"` // HTTP or TCP
	SSLConfig      string `json:"sslConfiguration"`
}

// CreateListener adds a listener to a load balancer
func (c *Client) CreateListener(loadBalancerID string, listener *Listener) bool {
	loadBalancerID = url.PathEscape(loadBalancerID)
	resp, err := c.Client.Request("POST", fmt.Sprintf("/loadBalancers/%s/listeners", loadBalancerID), *listener)
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

// UpdateListener updates a listener from a load balancer
func (c *Client) UpdateListener(listener *Listener) {
	// PUT loadBalancers/{loadBalancerId}/listeners/{listenerName}
	logrus.Warning("Method not yet implemented")
	return
}

// DeleteListener deletes a listener from a load balancer
func (c *Client) DeleteListener(listener *Listener) {
	// DELETE loadBalancers/{loadBalancerId}/listeners/{listenerName}
	logrus.Warning("Method not yet implemented")
	return
}
