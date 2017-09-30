package loadbalancer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"

	"github.com/Sirupsen/logrus"
)

// LoadBalancer represents the listener's configuration: https://docs.us-phoenix-1.oraclecloud.com/api/#/en/loadbalancer/20170115/LoadBalancer/
type LoadBalancer struct {
	// A mapping of strings to BackendSet objects
	BackendSets map[string]BackendSet `json:"backendSets,omitempty"`
	// A user-friendly name
	DisplayName string `json:"displayName"`
	// A mapping of strings to Certificate objects
	Certificates map[string]Certificate `json:"certificates,omitempty"`
	// The OCID of the compartment containing the load balancer
	CompartmentID string `json:"compartmentId"`
	// The OCID of the load balancer
	ID string `json:"id"`
	// An array of IP addresses
	IPAddresses []IP `json:"ipAddresses"`
	// Whether the load balancer has a VCN-local (private) IP address
	Private bool `json:"isPrivate"`
	// The current state of the instance.
	// CREATING | ACTIVE
	// FAILED | DELETING | DELETED
	LifeCycleState string `json:"lifecycleState"`
	// A mapping of strings to Listener objects
	Listeners map[string]Listener `json:"listeners"`
	// A template that determines the total pre-provisioned bandwidth
	ShapeName string `json:"shapeName"` // HTTP or TCP
	// An array of subnet OCIDs
	SubnetIDs []string `json:"subnetIds"`
	// The date and time the instance was created (RFC3339)
	TimeCreated string `json:"timeCreated"`
}

// IP is a load balancer IP address
type IP struct {
	Address string `json:"ipAddress"`
	Public  bool   `json:"isPublic"`
}

// GetLoadBalancer gets the specified load balancer's configuration information
func (c *Client) GetLoadBalancer(loadBalancerID string) (LoadBalancer, error) {
	loadBalancer := LoadBalancer{}
	loadBalancerID = url.PathEscape(loadBalancerID)
	resp, err := c.Request("GET", fmt.Sprintf("/loadBalancers/%s", loadBalancerID), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return loadBalancer, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return loadBalancer, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}

	if err = json.Unmarshal(body, &loadBalancer); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return loadBalancer, nil
}

// ListLoadBalancers lists all load balancers in the specified compartment
func (c *Client) ListLoadBalancers() ([]LoadBalancer, *bmc.Error) {
	loadBalancers := []LoadBalancer{}
	resp, err := c.Request("GET", fmt.Sprintf("/loadBalancers"), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return loadBalancers, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return loadBalancers, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}

	if err = json.Unmarshal(body, &loadBalancers); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return loadBalancers, nil
}
