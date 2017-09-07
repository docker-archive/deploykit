package compute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type apiClient struct {
	apiEndpoint *url.URL
	httpClient  *http.Client
}

func (c *apiClient) requestAndCheckStatus(description string, req *http.Request) (*http.Response, error) {
	rsp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if 200 <= rsp.StatusCode && rsp.StatusCode < 300 {
		return rsp, nil
	}

	return nil, unexpectedStatusError(description, rsp)
}

// UnexpectedStatusError is an Error returned when an HTTP API request returns a non-2** status code.
type UnexpectedStatusError struct {
	Description string
	Status      string
	StatusCode  int
	Body        string
}

// Error formats an UnexpectedStatusError as a string.
func (e UnexpectedStatusError) Error() string {
	return fmt.Sprintf("Unable to %s: %s\n%s", e.Description, e.Status, e.Body)
}

// WasNotFoundError detects whether an error was an UnexpectedStatusError triggered by a 404 status code.
func WasNotFoundError(e error) bool {
	err, ok := e.(UnexpectedStatusError)
	if ok {
		return err.StatusCode == 404
	}
	return false
}

func unexpectedStatusError(description string, rsp *http.Response) error {
	var bodyString string
	if rsp.Body == nil {
		bodyString = "<empty body>"
	} else {
		buf := new(bytes.Buffer)
		buf.ReadFrom(rsp.Body)
		bodyString = buf.String()
	}

	return UnexpectedStatusError{
		Description: description,
		Status:      rsp.Status,
		StatusCode:  rsp.StatusCode,
		Body:        bodyString,
	}
}

func (c *apiClient) newPostRequest(path string, body interface{}) (*http.Request, error) {
	req, err := c.newRequest("POST", path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/oracle-compute-v3+json")

	return req, nil
}

func (c *apiClient) newPutRequest(path string, body interface{}) (*http.Request, error) {
	req, err := c.newRequest("PUT", path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/oracle-compute-v3+json")

	return req, nil
}

func (c *apiClient) newGetRequest(path string) (*http.Request, error) {
	return c.newRequest("GET", path, nil)
}

func (c *apiClient) newDeleteRequest(path string) (*http.Request, error) {
	return c.newRequest("DELETE", path, nil)
}

func (c *apiClient) newRequest(method, path string, body interface{}) (*http.Request, error) {
	urlPath, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(
		method,
		c.apiEndpoint.ResolveReference(urlPath).String(),
		marshalToReader(body),
	)

	if err != nil {
		return nil, err
	}

	return req, nil
}

func marshalToReader(body interface{}) io.Reader {
	if body == nil {
		return nil
	}
	bodyData, err := json.Marshal(body)
	if err != nil {
		log.Panic(err)
	}
	return bytes.NewReader(bodyData)
}

func unmarshalResponseBody(rsp *http.Response, iface interface{}) error {
	buf := new(bytes.Buffer)
	buf.ReadFrom(rsp.Body)
	return json.Unmarshal(buf.Bytes(), iface)
}

func waitFor(description string, timeoutSeconds int, test func() (bool, error)) error {
	tick := time.Tick(1 * time.Second)

	for i := 0; i < timeoutSeconds; i++ {
		select {
		case <-tick:
			completed, err := test()
			if err != nil || completed {
				return err
			}
		}
	}
	return fmt.Errorf("Timeout waiting for %s", description)
}
