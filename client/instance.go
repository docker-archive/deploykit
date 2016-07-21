package client

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/instance"
	"io/ioutil"
	"net/http"
	"strings"
)

// NewInstanceProvisioner creates a remote provisioner and communicates using the HTTP API.
func NewInstanceProvisioner(host string) instance.Provisioner {
	return &instanceClient{host: host}
}

type instanceClient struct {
	host string
}

func (c instanceClient) sendRequest(method, path, body string) (*http.Response, error) {
	req, err := http.NewRequest(method, fmt.Sprintf("http://%s/%s", c.host, path), strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if (resp.StatusCode / 100) != 2 {
		// TODO(wfarner): Reverse-map HTTP status codes to spi.Error codes for better error handling, then
		// return spi.Error
		data, _ := ioutil.ReadAll(resp.Body)
		return nil, spi.NewError(spi.ErrUnknown, string(data))
	}

	return resp, nil
}

func (c instanceClient) Provision(request string) (*instance.ID, error) {
	resp, apiErr := c.sendRequest("POST", "instance/", request)
	if apiErr != nil {
		return nil, apiErr
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	id := instance.ID(string(data))
	return &id, nil
}

func (c instanceClient) Destroy(instance instance.ID) error {
	_, apiErr := c.sendRequest("DELETE", fmt.Sprintf("instance/%s", instance), "")
	return apiErr
}

func (c instanceClient) ListGroup(group instance.GroupID) ([]instance.ID, error) {
	resp, apiErr := c.sendRequest("GET", fmt.Sprintf("instance/?group=%s", group), "")
	if apiErr != nil {
		return nil, apiErr
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	ids := []instance.ID{}
	err = json.Unmarshal(data, &ids)
	if err != nil {
		return nil, err
	}

	return ids, nil
}
