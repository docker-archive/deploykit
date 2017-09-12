package compute

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/google/go-querystring/query"
)

// InstanceClient is a client for the Instance functions of the Compute API.
type InstanceClient struct {
	client        *APIClient
	compartmendID string
}

// Instance contains the instance reference from:
// https://docs.us-phoenix-1.oraclecloud.com/api/#/en/iaas/20160918/Instance/
type Instance struct {
	AvailabilityDomain string `json:"availabilityDomain"`
	CompartmentID      string `json:"compartmentId"`
	DisplayName        string `json:"displayName"`
	ExtendedMetadata   struct {
	} `json:"extendedMetadata"`
	ID             string `json:"id"`
	ImageID        string `json:"imageId"`
	IpxeScript     string `json:"ipxeScript"`
	LifecycleState string `json:"lifecycleState"`
	Metadata       struct {
	} `json:"metadata"`
	Region      string `json:"region"`
	Shape       string `json:"shape"`
	TimeCreated string `json:"timeCreated"`
}

// InstancesParameters
type InstancesParameters struct {
	AvailabilityDomain string `url:"availabilityDomain,omitempty"` //The name of the Availability Domain.
	DisplayName        string `url:"displayName,omitempty"`        //A user-friendly name. Does not have to be unique, and it's changeable. Avoid entering confidential information.
	Limit              int    `url:"limit,omitempty"`              //The maximum number of items to return in a paginated "List" call.
	Page               string `url:"page,omitempty"`               //The value of the opc-next-page response header from the previous "List" call
}

// NewInstanceClient provides a client interface for instance API calls
func (c *APIClient) NewInstanceClient(compartment string) *InstanceClient {
	return &InstanceClient{
		client:        c,
		compartmendID: compartment,
	}
}

// GetInstance returns a struct of an instance request given an instance ID
func (ic *InstanceClient) GetInstance(instanceID string) Instance {
	instance := Instance{}
	resp, err := ic.client.Get("/instances/" + instanceID)
	if err != nil {
		logrus.Error(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}
	if err = json.Unmarshal(body, &instance); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return instance
}

// ListInstances returns a slice struct of all instance
func (ic *InstanceClient) ListInstances(options *InstancesParameters) {
	queryString := url.QueryEscape(ic.compartmendID)
	if options != nil {
		v, _ := query.Values(*options)
		queryString = queryString + "&" + v.Encode()
	}
	resp, err := ic.client.Get(fmt.Sprintf("/instances?compartmentId=%s", queryString))
	if err != nil {
		logrus.Error(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}
	logrus.Info(string(body))
	// if err = json.Unmarshal(body, &instance); err != nil {
	// 	logrus.Fatalf("Unmarshal impossible: %s", err)
	// }
	return
}
