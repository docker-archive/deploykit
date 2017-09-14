package compute

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/Sirupsen/logrus"
)

// VNicClient is a client for the Instance functions of the Compute API.
type VNicClient struct {
	client        *APIClient
	compartmendID string
}

// VNic contains the VNIC reference from: https://docs.us-phoenix-1.oraclecloud.com/api/#/en/iaas/20160918/Vnic/
type VNic struct {
	// The Availability Domain the instance is running in
	AvailabilityDomain string `json:"availabilityDomain"`
	// The OCID of the compartment that contains the instance
	CompartmentID string `json:"compartmentId"`
	// A user-friendly name
	DisplayName string `json:"displayName"`
	// The hostname for the VNIC's primary private IP
	HostnameLabel string `json:"hostnameLabel"`
	// The OCID of the VNIC
	ID string `json:"id"`
	// Whether the VNIC is the primary VNIC
	Primary bool `json:"isPrimary"`
	// The current state of the instance.
	// PROVISIONING | AVAILABLE
	// TERMINATING | TERMINATED
	LifeCycleState string `json:"lifecycleState"`
	// The MAC address of the VNIC
	MacAddress string `json:"macAddress"`
	// The private IP address of the primary privateIp object on the VNIC
	PrivateIP string `json:"privateIp"`
	// The public IP address of the VNIC, if one is assigned.
	PublicIP string `json:"publicIp"`
	// Whether the source/destination check is disabled on the VNIC
	SrcDestCheck bool `json:"skipSourceDestCheck"`
	// The OCID of the subnet the VNIC is in
	SubnetID string `json:"subnetId"`
	// The date and time the instance was created (RFC3339)
	TimeCreated string `json:"timeCreated"`
}

// VNicAttachment contains the VNICAttachement reference from: https://docs.us-phoenix-1.oraclecloud.com/api/#/en/iaas/20160918/VnicAttachment/
type VNicAttachment struct {
	// The Availability Domain the instance is running in
	AvailabilityDomain string `json:"availabilityDomain"`
	// The OCID of the compartment that contains the instance
	CompartmentID string `json:"compartmentId"`
	// A user-friendly name
	DisplayName string `json:"displayName"`
	// The OCID of the VNIC
	ID string `json:"id"`
	// The OCID of the instance
	InstanceID string `json:"instanceId"`
	// The current state of the instance.
	// ATTACHING | ATTACHED
	// DETACHING | DETACHED
	LifeCycleState string `json:"lifecycleState"`
	// The OCID of the subnet the VNIC is in
	SubnetID string `json:"subnetId"`
	// The date and time the instance was created (RFC3339)
	TimeCreated string `json:"timeCreated"`
	// The Oracle-assigned VLAN tag of the attached VNIC
	VlanTag int `json:"vlanTag"`
	// The OCID of the VNIC
	VNicID string `json:"vnicId"`
}

// NewVNicClient provides a client interface for VNic API calls
func (c *APIClient) NewVNicClient(compartment string) *VNicClient {
	return &VNicClient{
		client:        c,
		compartmendID: compartment,
	}
}

// GetVNicAttachment returns a struct of a VNicAttachment request given an VNicAttachment ID
func (vc *VNicClient) GetVNicAttachment(vNicAttachmentID string) VNicAttachment {
	vNicAttachment := VNicAttachment{}
	queryString := url.QueryEscape(vNicAttachmentID)
	resp, err := vc.client.Get("/vnicAttachments/" + queryString)
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
	if err = json.Unmarshal(body, &vNicAttachment); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return vNicAttachment
}

// ListVNicAttachments returns a slice struct of all instance
func (vc *VNicClient) ListVNicAttachments(instanceID string) []VNicAttachment {
	vNicAttachments := []VNicAttachment{}
	queryString := url.QueryEscape(vc.compartmendID)
	if instanceID != "" {
		queryString = queryString + "&" + url.QueryEscape(instanceID)
	}
	resp, err := vc.client.Get(fmt.Sprintf("/vnicAttachments?compartmentId=%s", queryString))
	if err != nil {
		logrus.Error(err)
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}
	logrus.Debug("Body: ", string(body))
	if err = json.Unmarshal(body, &vNicAttachments); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return vNicAttachments
}

// GetVNic returns a struct of a VNic request given an VNic ID
func (vc *VNicClient) GetVNic(vnicID string) VNic {
	vnic := VNic{}
	queryString := url.QueryEscape(vnicID)
	resp, err := vc.client.Get("/vnics/" + queryString)
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
	if err = json.Unmarshal(body, &vnic); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return vnic
}

// ListVNic returns all VNic associated with an instance ID
func (vc *VNicClient) ListVNic(instanceID string) []VNic {
	vNicAttachments := vc.ListVNicAttachments(instanceID)
	vNics := []VNic{}
	for _, vNicAttachment := range vNicAttachments {
		vNics = append(vNics, vc.GetVNic(vNicAttachment.VNicID))
	}
	return vNics
}
