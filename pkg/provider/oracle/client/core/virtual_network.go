package core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"
)

// VCN contains the VCN reference from: https://docs.us-phoenix-1.oraclecloud.com/api/#/en/iaas/20160918/Vcn/
type VCN struct {
	// The Availability Domain the instance is running in
	CidrBlock string `json:"cidrBlock"`
	// The OCID of the compartment that contains the instance
	CompartmentID string `json:"compartmentId"`
	// A user-friendly name
	DisplayName string `json:"displayName"`
	// The OCID of the VNIC
	ID string `json:"id"`
	// The OCID for the VCN's default set of DHCP options
	DefaultDhcpOptionsID string `json:"defaultDhcpOptionsId"`
	// The OCID for the VCN's default route table
	DefaultRouteTableID string `json:"defaultRouteTableId"`
	// The OCID for the VCN's default security list
	DefaultSecurityListID string `json:"defaultSecurityListId"`
	// The VCN's current state
	LifeCycleState string `json:"lifecycleState"`
	// A DNS label for the VCN
	DNSlabel string `json:"dnsLabel"`
	// The date and time the instance was created (RFC3339)
	TimeCreated string `json:"timeCreated"`
	// The VCN's domain name
	VcnDomainName int `json:"vcnDomainName"`
	// The OCID of the VNIC
	VNicID string `json:"vnicId"`
}

// GetVcn returns a struct of a VCN request given an VCN ID
func (c *Client) GetVcn(vcnID string) (VCN, *bmc.Error) {
	vcn := VCN{}
	queryString := url.QueryEscape(vcnID)
	resp, err := c.Request("GET", "/vcns/"+queryString, nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return vcn, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return vcn, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}
	if err = json.Unmarshal(body, &vcn); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return vcn, nil
}

// ListVcns returns a slice struct of all vcn
func (c *Client) ListVcns() ([]VCN, *bmc.Error) {
	vcns := []VCN{}
	queryString := url.QueryEscape(c.CompartmentID)
	resp, err := c.Request("GET", fmt.Sprintf("/vcns?compartmentId=%s", queryString), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return vcns, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return vcns, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}

	logrus.Debug("Body: ", string(body))
	if err = json.Unmarshal(body, &vcns); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return vcns, nil
}
