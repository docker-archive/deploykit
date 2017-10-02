package loadbalancer

import (
	"fmt"
	"net/url"

	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"

	"github.com/Sirupsen/logrus"
)

// Listener represents the listener's configuration: https://docs.us-phoenix-1.oraclecloud.com/api/#/en/loadbalancer/20170115/Listener/
type Listener struct {
	BackendSetName string            `json:"defaultBackendSetName"`
	Name           string            `json:"name"`
	Port           int               `json:"port"`
	Protocol       string            `json:"protocol"` // HTTP or TCP
	SSLConfig      *SSLConfiguration `json:"sslConfiguration,omitempty"`
}

// CreateListener adds a listener to a load balancer
func (c *Client) CreateListener(loadBalancerID string, listener *Listener) (bool, *bmc.Error) {
	loadBalancerID = url.PathEscape(loadBalancerID)
	resp, err := c.Request("POST", fmt.Sprintf("/loadBalancers/%s/listeners", loadBalancerID), *listener)
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

// UpdateListener updates a listener from a load balancer
func (c *Client) UpdateListener(listener *Listener) {
	// PUT loadBalancers/{loadBalancerId}/listeners/{listenerName}
	logrus.Warning("Method not yet implemented")
	return
}

// DeleteListener deletes a listener from a load balancer
func (c *Client) DeleteListener(loadBalancerID string, listenerName string) (bool, *bmc.Error) {
	loadBalancerID = url.PathEscape(loadBalancerID)
	listenerName = url.PathEscape(listenerName)
	resp, err := c.Request("DELETE", fmt.Sprintf("/loadBalancers/%s/listeners/%s", loadBalancerID, listenerName), nil)
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
