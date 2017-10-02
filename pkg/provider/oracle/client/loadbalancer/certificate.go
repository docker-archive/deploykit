package loadbalancer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"

	"github.com/Sirupsen/logrus"
)

// Certificate represents the listener's configuration: https://docs.us-phoenix-1.oraclecloud.com/api/#/en/loadbalancer/20170115/Certificate/
type Certificate struct {
	// A friendly name for the certificate bundle
	CertificateName string `json:"certificateName"`
	// The Certificate Authority certificate
	CACertificates string `json:"caCertificate,omitempty"`
	// The public certificate, in PEM format
	PublicCertificate string `json:"publicCertificate"`
}

// ListCertificates lists all load balancers in the specified compartment
func (c *Client) ListCertificates(loadBalancerID string) ([]Certificate, *bmc.Error) {
	certificates := []Certificate{}
	loadBalancerID = url.PathEscape(loadBalancerID)
	resp, err := c.Request("GET", fmt.Sprintf("/loadBalancers/%s/certificates", loadBalancerID), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return certificates, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return certificates, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}

	if err = json.Unmarshal(body, &certificates); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return certificates, nil
}
