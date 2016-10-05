package aws

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// MetadataKey is the identifier for a metadata entry.
type MetadataKey string

const (
	// MetadataAmiID - AMI ID
	MetadataAmiID = MetadataKey("http://169.254.169.254/latest/meta-data/ami-id")

	// MetadataInstanceID - Instance ID
	MetadataInstanceID = MetadataKey("http://169.254.169.254/latest/meta-data/instance-id")

	// MetadataMAC - the mac of eth0
	MetadataMAC = MetadataKey("http://169.254.169.254/latest/meta-data/mac")

	// MetadataInstanceType - Instance type
	MetadataInstanceType = MetadataKey("http://169.254.169.254/latest/meta-data/instance-type")

	// MetadataHostname - Host name
	MetadataHostname = MetadataKey("http://169.254.169.254/latest/meta-data/hostname")

	// MetadataLocalIPv4 - Local IPv4 address
	MetadataLocalIPv4 = MetadataKey("http://169.254.169.254/latest/meta-data/local-ipv4")

	// MetadataPublicIPv4 - Public IPv4 address
	MetadataPublicIPv4 = MetadataKey("http://169.254.169.254/latest/meta-data/public-ipv4")

	// MetadataAvailabilityZone - Availability zone
	MetadataAvailabilityZone = MetadataKey("http://169.254.169.254/latest/meta-data/placement/availability-zone")

	// Formats for building keys that are dependent on mac address
	metadataSubnetID         = "http://169.254.169.254/latest/meta-data/network/interfaces/macs/%s/subnet-id"
	metadataSecurityGroupIDs = "http://169.254.169.254/latest/meta-data/network/interfaces/macs/%s/security-group-ids"
)

// MetadataSubnetID returns the subnet id, which depends on the mac of the instance
func MetadataSubnetID() (string, error) {
	mac, err := GetMetadata(MetadataMAC)
	if err != nil {
		return "", err
	}
	return GetMetadata(MetadataKey(fmt.Sprintf(metadataSubnetID, mac)))
}

// MetadataSecurityGroupIDS returns the subnet id, which depends on the mac of the instance
func MetadataSecurityGroupIDs() ([]string, error) {
	mac, err := GetMetadata(MetadataMAC)
	if err != nil {
		return nil, err
	}
	str, err := GetMetadata(MetadataKey(fmt.Sprintf(metadataSecurityGroupIDs, mac)))
	if err != nil {
		return nil, err
	}
	return strings.Split(str, "\n"), nil
}

// GetMetadata returns the value of the metadata by key
func GetMetadata(key MetadataKey) (string, error) {
	resp, err := http.Get(string(key))
	if err != nil {
		return "", err
	}
	buff, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", err
	}
	return string(buff), nil
}

// GetRegion returns the AWS region this instance is in.
func GetRegion() (string, error) {
	az, err := GetMetadata(MetadataAvailabilityZone)
	if err != nil {
		return "", err
	}
	return az[0 : len(az)-1], nil
}
