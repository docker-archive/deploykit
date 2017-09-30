package core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"
	"github.com/google/go-querystring/query"
)

// Instance contains the instance reference from:
// https://docs.us-phoenix-1.oraclecloud.com/api/#/en/iaas/20160918/Instance/
type Instance struct {
	// The Availability Domain the instance is running in
	AvailabilityDomain string `json:"availabilityDomain"`
	// The OCID of the compartment that contains the instance
	CompartmentID string `json:"compartmentId"`
	// A user-friendly name
	DisplayName string `json:"displayName"`
	// Additional metadata key/value pairs that you provide
	ExtendedMetadata *json.RawMessage
	// The OCID of the instance
	ID string `json:"id"`
	// The image used to boot the instance
	ImageID string `json:"imageId"`
	// iPXE script to continue the boot process.
	IpxeScript string `json:"ipxeScript"`
	// The current state of the instance.
	// PROVISIONING | RUNNING | STARTING |
	// STOPPING | STOPPED | CREATING_IMAGE | TERMINATING | TERMINATED
	LifeCycleState string `json:"lifecycleState"`
	// Custom metadata that you provide
	Metadata struct {
		PublicKey string `json:"ssh_authorized_keys"`
		UserData  string `json:"user_data"`
	} `json:"metadata"`
	// The region that contains the Availability Domain the instance is running in
	Region string `json:"region"`
	// The shape of the instance
	Shape string `json:"shape"`
	// The date and time the instance was created (RFC3339)
	TimeCreated string `json:"timeCreated"`
}

// InstancesParameters are optional parameters when listing instances
type InstancesParameters struct {
	AvailabilityDomain string `url:"availabilityDomain,omitempty"` //The name of the Availability Domain.
	DisplayName        string `url:"displayName,omitempty"`        //A user-friendly name. Does not have to be unique, and it's changeable. Avoid entering confidential information.
	Limit              int    `url:"limit,omitempty"`              //The maximum number of items to return in a paginated "List" call.
	Page               string `url:"page,omitempty"`               //The value of the opc-next-page response header from the previous "List" call
	Filter             *InstanceFilter
}

// GetInstance returns a struct of an instance request given an instance ID
func (c *Client) GetInstance(instanceID string) (Instance, *bmc.Error) {
	instance := Instance{}
	queryString := url.QueryEscape(instanceID)
	resp, err := c.Request("GET", "/instances/"+queryString, nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return instance, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return instance, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}
	if err = json.Unmarshal(body, &instance); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return instance, nil
}

// ListInstances returns a slice struct of all instance
func (c *Client) ListInstances(options *InstancesParameters) ([]Instance, error) {
	instances := []Instance{}
	queryString := url.QueryEscape(c.CompartmentID)
	if options != nil {
		v, _ := query.Values(*options)
		queryString = queryString + "&" + v.Encode()
	}
	resp, err := c.Request("GET", fmt.Sprintf("/instances?compartmentId=%s", queryString), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return instances, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return instances, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}

	if err = json.Unmarshal(body, &instances); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	if options.Filter != nil {
		instances = filterInstances(instances, *options.Filter)
	}
	return instances, nil
}
