package core

import (
	"encoding/json"
	"io/ioutil"
	"net/url"

	"github.com/FrenchBen/oracle-sdk-go/compute"
	"github.com/Sirupsen/logrus"
)

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

// GetVNic returns a struct of a VNic request given an VNic ID
func (c *VNicClient) GetVNic(vnicID string) VNic {
	vnic := VNic{}
	queryString := url.QueryEscape(vnicID)
	resp, err := c.Client.Get("/vnics/" + queryString)
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
func (c *VNicClient) ListVNic(instanceID string) []VNic {
	vNicAttachments := vn.ListVNicAttachments(instanceID)
	vNics := []VNic{}
	for _, vNicAttachment := range vNicAttachments {
		vNics = append(vNics, vn.GetVNic(vNicAttachment.VNicID))
	}
	return vNics
}
