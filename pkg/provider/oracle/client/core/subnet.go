package core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"
)

// Subnet contains the Subnet reference from: https://docs.us-phoenix-1.oraclecloud.com/api/#/en/iaas/20160918/Subnet/
type Subnet struct {
	// The subnet's Availability Domain
	AvailabilityDomain string `json:"availabilityDomain"`
	// The subnet's CIDR block
	CidrBlock string `json:"cidrBlock"`
	// The OCID of the compartment that contains the instance
	CompartmentID string `json:"compartmentId"`
	// A user-friendly name
	DisplayName string `json:"displayName"`
	// The OCID of the VNIC
	ID string `json:"id"`
	// The OCID of the set of DHCP options associated with the subnet
	DhcpOptionsID string `json:"dhcpOptionsId"`
	// ProhibitPublicIPOnVnic sets whether VNICs within this subnet can have public IP addresses
	ProhibitPublicIPOnVnic bool `json:"prohibitPublicIpOnVnic"`
	// RouteTableID is the OCID of the route table the subnet is using
	RouteTableID string `json:"routeTableId"`
	// The Subnet's current state
	LifeCycleState string `json:"lifecycleState"`
	// A DNS label for the Subnet
	DNSlabel string `json:"dnsLabel"`
	// The date and time the instance was created (RFC3339)
	TimeCreated string `json:"timeCreated"`
	// OCIDs for the security lists to use for VNICs in this subnet
	SecurityListIDs []string `json:"securityListIds"`
	// The OCID of the VNIC
	VNicID string `json:"vnicId"`
	// The subnet's domain name
	SubnetDomainName string `json:"subnetDomainName"`
	// The IP address of the virtual router
	VirtualRouterIP string `json:"virtualRouterIp"`
	// The MAC address of the virtual router
	VirtualRouterMac string `json:"virtualRouterMac"`
}

// GetSubnet returns a struct of a Subnet request given an Subnet ID
func (c *Client) GetSubnet(subnetID string) (Subnet, *bmc.Error) {
	subnet := Subnet{}
	queryString := url.QueryEscape(subnetID)
	resp, err := c.Request("GET", "/subnets/"+queryString, nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return subnet, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return subnet, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}
	if err = json.Unmarshal(body, &subnet); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return subnet, nil
}

// ListSubnets returns a slice struct of all subnet
func (c *Client) ListSubnets() ([]Subnet, *bmc.Error) {
	subnets := []Subnet{}
	queryString := url.QueryEscape(c.CompartmentID)
	resp, err := c.Request("GET", fmt.Sprintf("/subnets?compartmentId=%s", queryString), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return subnets, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return subnets, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}

	logrus.Debug("Body: ", string(body))
	if err = json.Unmarshal(body, &subnets); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return subnets, nil
}
