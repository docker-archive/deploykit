package core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/Sirupsen/logrus"
)

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

// GetVNicAttachment returns a struct of a VNicAttachment request given an VNicAttachment ID
func (c *CoreClient) GetVNicAttachment(vNicAttachmentID string) VNicAttachment {
	vNicAttachment := VNicAttachment{}
	queryString := url.QueryEscape(vNicAttachmentID)
	resp, err := c.Client.Get("/vnicAttachments/" + queryString)
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
func (c *CoreClient) ListVNicAttachments(instanceID string) []VNicAttachment {
	vNicAttachments := []VNicAttachment{}
	queryString := url.QueryEscape(c.CompartmentID)
	if instanceID != "" {
		queryString = queryString + "&" + url.QueryEscape(instanceID)
	}
	resp, err := c.Client.Get(fmt.Sprintf("/vnicAttachments?compartmentId=%s", queryString))
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
