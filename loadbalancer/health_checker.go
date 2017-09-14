package lb

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/Sirupsen/logrus"
)

// HealthChecker reference from https://docs.us-phoenix-1.oraclecloud.com/api/#/en/loadbalancer/20170115/HealthChecker/
type HealthChecker struct {
	Interval   int    `json:"intervalInMillis"`
	Port       string `json:"port"`
	Protocol   string `json:"protocol"`
	Response   string `json:"responseBodyRegex"`
	Retries    int    `json:"retries"`
	ReturnCode int    `json:"returnCode"`
	Timeout    int    `json:"timeoutInMillis"`
	URLPath    string `json:"urlPath"`
}

// GetHealthChecker gets the health check policy information for a given load balancer and backend set.
func (c *Client) GetHealthChecker(loadBalancerID string, backendSetName string) HealthChecker {
	healthChecker := HealthChecker{}
	loadBalancerID = url.PathEscape(loadBalancerID)
	backendSetName = url.PathEscape(backendSetName)
	resp, err := c.Client.Request("GET", fmt.Sprintf("/loadBalancers/%s/backendSets/%s/healthChecker", loadBalancerID, backendSetName), nil)
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
	if err = json.Unmarshal(body, &healthChecker); err != nil {
		logrus.Fatalf("Unmarshal impossible: %s", err)
	}
	return healthChecker
}
