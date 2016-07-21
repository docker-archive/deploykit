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

func (c instanceClient) sendRequest(method, path, body string) ([]byte, error) {
	req, err := http.NewRequest(method, fmt.Sprintf("http://%s/%s", c.host, path), strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	var data []byte
	if resp != nil {
		var readErr error
		data, readErr = ioutil.ReadAll(resp.Body)
		if readErr != nil {
			data = nil
			err = readErr
		} else if (resp.StatusCode / 100) != 2 {
			// Attempt to map the body into the server's error structure,  making the return values
			// identical to those from the SPI implementation.
			errorStruct := map[string]string{}
			unmarshalErr := json.Unmarshal(data, &errorStruct)
			if unmarshalErr == nil && errorStruct["error"] != "" {
				data = nil

				// TODO(wfarner): Reverse-map HTTP status codes to spi.Error codes for better error
				// handling.
				err = spi.NewError(spi.ErrUnknown, errorStruct["error"])
			} else {
				err = spi.NewError(spi.ErrUnknown, "")
			}
		}
	}

	return data, err
}

func (c instanceClient) Provision(request string) (*instance.ID, error) {
	data, apiErr := c.sendRequest("POST", "instance/", request)
	if apiErr != nil {
		return nil, apiErr
	}

	var idString string
	err := json.Unmarshal(data, &idString)
	if err != nil {
		return nil, err
	}

	id := instance.ID(idString)
	return &id, nil
}

func (c instanceClient) Destroy(instance instance.ID) error {
	_, apiErr := c.sendRequest("DELETE", fmt.Sprintf("instance/%s", instance), "")
	return apiErr
}

func (c instanceClient) ListGroup(group instance.GroupID) ([]instance.ID, error) {
	data, apiErr := c.sendRequest("GET", fmt.Sprintf("instance/?group=%s", group), "")
	if apiErr != nil {
		return nil, apiErr
	}

	ids := []instance.ID{}
	err := json.Unmarshal(data, &ids)
	if err != nil {
		return nil, err
	}

	return ids, nil
}

func (c instanceClient) ShellExec(id instance.ID, shellCode string) (*string, error) {
	// When valid response data is available, always return it along with the error.
	data, apiErr := c.sendRequest("PUT", fmt.Sprintf("instance/%s/exec", id), shellCode)
	if data == nil {
		return nil, apiErr
	}

	var dataString string
	unmarshalErr := json.Unmarshal(data, &dataString)

	var returnErr error
	switch {
	case apiErr != nil:
		returnErr = apiErr
	case unmarshalErr != nil:
		returnErr = unmarshalErr
	}

	return &dataString, returnErr
}
