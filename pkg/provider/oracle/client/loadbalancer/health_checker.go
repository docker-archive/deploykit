package loadbalancer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"
)

// HealthChecker reference from https://docs.us-phoenix-1.oraclecloud.com/api/#/en/loadbalancer/20170115/HealthChecker/
type HealthChecker struct {
	Interval   int    `json:"intervalInMillis,omitempty"`
	Port       int    `json:"port,omitempty"`
	Protocol   string `json:"protocol"`                    // HTTP or TCP
	Response   string `json:"responseBodyRegex,omitempty"` // ^(500|40[1348])$
	Retries    int    `json:"retries,omitempty"`
	ReturnCode int    `json:"returnCode,omitempty"`
	Timeout    int    `json:"timeoutInMillis,omitempty"`
	URLPath    string `json:"urlPath"`
}

// GetHealthChecker gets the health check policy information for a given load balancer and backend set.
func (c *Client) GetHealthChecker(loadBalancerID string, backendSetName string) (HealthChecker, *bmc.Error) {
	healthChecker := HealthChecker{}
	loadBalancerID = url.PathEscape(loadBalancerID)
	backendSetName = url.PathEscape(backendSetName)
	resp, err := c.Request("GET", fmt.Sprintf("/loadBalancers/%s/backendSets/%s/healthChecker", loadBalancerID, backendSetName), nil)
	if err != nil {
		logrus.Error(err)
		bmcError := bmc.Error{Code: string(resp.StatusCode), Message: err.Error()}
		return healthChecker, &bmcError
	}
	logrus.Debug("StatusCode: ", resp.StatusCode)
	if resp.StatusCode != 200 {
		return healthChecker, bmc.NewError(*resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debug("Body: ", string(body))
	if err != nil {
		logrus.Fatalf("Could not read JSON response: %s", err)
	}

	if err = json.Unmarshal(body, &healthChecker); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return healthChecker, nil
}
