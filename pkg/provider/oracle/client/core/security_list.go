package core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"
)

// SecurityList contains the SecurityList reference from: https://docs.us-phoenix-1.oraclecloud.com/api/#/en/iaas/20160918/SecurityList/
type SecurityList struct {
	// The OCID of the compartment that contains the security list
	CompartmentID string `json:"compartmentId,omitempty"`
	// A user-friendly name
	DisplayName string `json:"displayName,omitempty"`
	// The OCID of the SecurityList
	ID string `json:"id,omitempty"`
	// Rules for allowing egress IP packets
	EgressSecurityRules *[]EgressSecurityRule `json:"egressSecurityRules"`
	// Rules for allowing ingress IP packets
	IngressSecurityRules *[]IngressSecurityRule `json:"ingressSecurityRules"`
	// The SecurityList's current state
	LifeCycleState string `json:"lifecycleState,omitempty"`
	// The date and time the instance was created (RFC3339)
	TimeCreated string `json:"timeCreated,omitempty"`
	// The OCID of the VCN
	VcnID string `json:"vcnId,omitempty"`
}

// SecurityListUpdate message body for updating security list
type SecurityListUpdate struct {
	// A user-friendly name
	DisplayName string `json:"displayName,omitempty"`
	// Rules for allowing egress IP packets
	EgressSecurityRules *[]EgressSecurityRule `json:"egressSecurityRules"`
	// Rules for allowing ingress IP packets
	IngressSecurityRules *[]IngressSecurityRule `json:"ingressSecurityRules"`
}

// EgressSecurityRule rule for allowing outbound IP packets
type EgressSecurityRule struct {
	Destination string      `json:"destination"`
	IcmpOptions *IcmpOption `json:"icmpOptions,omitempty"`
	IsStateless bool        `json:"isStateless,omitempty"`
	// Protocol values: all, ICMP ("1"), TCP ("6"), UDP ("17").
	Protocol   string      `json:"protocol"`
	TCPOptions *PortConfig `json:"tcpOptions,omitempty"`
	UDPOptions *PortConfig `json:"udpOptions,omitempty"`
}

// IngressSecurityRule rule for allowing inbound IP packets
type IngressSecurityRule struct {
	Source      string      `json:"source"`
	IcmpOptions *IcmpOption `json:"icmpOptions,omitempty"`
	IsStateless bool        `json:"isStateless,omitempty"`
	// Protocol values: all, ICMP ("1"), TCP ("6"), UDP ("17").
	Protocol   string      `json:"protocol"`
	TCPOptions *PortConfig `json:"tcpOptions,omitempty"`
	UDPOptions *PortConfig `json:"udpOptions,omitempty"`
}

// IcmpOption settings for ICMP
type IcmpOption struct {
	// The ICMP code
	Code int `json:"code,omitempty"`
	// The ICMP type
	Type int `json:"type,omitempty"`
}

// PortConfig contains port ranges for source and destinations
type PortConfig struct {
	DestinationPortRange *PortRange `json:"destinationPortRange,omitempty"`
	SourcePortRange      *PortRange `json:"sourcePortRange,omitempty"`
}

// PortRange is a range of ports
type PortRange struct {
	Min int `json:"min,omitempty"`
	Max int `json:"max,omitempty"`
}

// GetSecurityList returns a struct of a SecurityList request given an SecurityList ID
func (c *Client) GetSecurityList(securityListID string) (SecurityList, *bmc.Error) {
	securityList := SecurityList{}
	queryString := url.QueryEscape(securityListID)
	resp, err := c.Request("GET", "/securityLists/"+queryString, nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return securityList, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return securityList, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}
	if err = json.Unmarshal(body, &securityList); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return securityList, nil
}

// ListSecurityLists returns a slice struct of all securityList
func (c *Client) ListSecurityLists(vcnID string) ([]SecurityList, *bmc.Error) {
	securityLists := []SecurityList{}
	queryString := url.QueryEscape(c.CompartmentID)
	if vcnID != "" {
		queryString = queryString + "&vcnId=" + url.QueryEscape(vcnID)
	}
	resp, err := c.Request("GET", fmt.Sprintf("/securityLists?compartmentId=%s", queryString), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return securityLists, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return securityLists, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}

	logrus.Debug("Body: ", string(body))
	if err = json.Unmarshal(body, &securityLists); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return securityLists, nil
}

// CreateSecurityList creates a new security list for the specified VCN
func (c *Client) CreateSecurityList(securityList *SecurityList) (bool, *bmc.Error) {
	resp, err := c.Request("POST", fmt.Sprintf("/securityLists/"), *securityList)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return false, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return false, bmc.NewError(*resp)
	}
	return true, nil
}

// UpdateSecurityList creates a new security list for the specified VCN
func (c *Client) UpdateSecurityList(securityListID string, securityListUpdate *SecurityListUpdate) (bool, *bmc.Error) {
	securityListID = url.PathEscape(securityListID)
	resp, err := c.Request("PUT", fmt.Sprintf("/securityLists/%s", securityListID), *securityListUpdate)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return false, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return false, bmc.NewError(*resp)
	}
	return true, nil
}

// DeleteSecurityList deletes the specified security list, but only if it's not associated with a subnet
func (c *Client) DeleteSecurityList(securityListID string) (bool, *bmc.Error) {
	securityListID = url.PathEscape(securityListID)
	resp, err := c.Request("DELETE", fmt.Sprintf("/securityLists/%s", securityListID), nil)
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
