package client

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/server/api"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"io/ioutil"
	"net/http"
	"strings"
)

// NewInstanceProvisioner creates a remote provisioner and communicates using the HTTP API.
func NewInstanceProvisioner(host string) instance.Plugin {
	return &instanceClient{host: host}
}

type instanceClient struct {
	host string
}

// Counterpart to the inverse map on the server side.
var httpStatusToSpiError = map[int]int{
	http.StatusBadRequest:          spi.ErrBadInput,
	http.StatusInternalServerError: spi.ErrUnknown,
	http.StatusConflict:            spi.ErrDuplicate,
	http.StatusNotFound:            spi.ErrNotFound,
}

func getSpiErrorCode(status int) int {
	code, mapped := httpStatusToSpiError[status]
	if !mapped {
		code = spi.ErrUnknown
	}
	return code
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
				err = spi.NewError(getSpiErrorCode(resp.StatusCode), errorStruct["error"])
			} else {
				err = spi.NewError(spi.ErrUnknown, "")
			}
		}
	}

	return data, err
}

func (c instanceClient) Provision(gid group.ID, request string, volume *instance.VolumeID) (*instance.ID, error) {
	req := json.RawMessage(request)
	payload := api.ProvisionRequest{
		Group:   gid,
		Request: &req,
		Volume:  volume,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	data, apiErr := c.sendRequest("POST", "instance/", string(body))
	if apiErr != nil {
		return nil, apiErr
	}

	var idString string
	err = json.Unmarshal(data, &idString)
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

func (c instanceClient) DescribeInstances(gid group.ID) ([]instance.Description, error) {
	data, apiErr := c.sendRequest("GET", fmt.Sprintf("instance/?group=%s", gid), "")
	if apiErr != nil {
		return nil, apiErr
	}

	descriptions := []instance.Description{}
	err := json.Unmarshal(data, &descriptions)
	if err != nil {
		return nil, err
	}

	return descriptions, nil
}
