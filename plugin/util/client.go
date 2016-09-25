package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/plugin"
)

// HTTPEndpoint is a specialization of an endpoint. It implements the Endpoint interface
type HTTPEndpoint struct {
	Method string
	Path   string
}

func (h *HTTPEndpoint) String() string {
	return "http:" + h.Method + ":" + h.Path
}

// GetHTTPEndpoint returns an http endpoint if the input endpoint is a supported http endpoint.
func GetHTTPEndpoint(endpoint plugin.Endpoint) (*HTTPEndpoint, error) {
	ep, ok := endpoint.(*HTTPEndpoint)
	if !ok {
		return nil, fmt.Errorf("unsupported endpoint: %v", endpoint)
	}

	if ep.Method == "" || ep.Path == "" {
		return nil, fmt.Errorf("invalid http endpoint:%v", endpoint)
	}
	return ep, nil
}

// Client is the client that can access the driver either via tcp or unix
type Client struct {
	endpoint *url.URL
	c        *http.Client
}

// newHTTPClient creates a http client given the listener address.  The address is in the form of a url.
// For example:  tcp://host:port or unix://path/to/socket/file
func newHTTPClient(listenerURL *url.URL) *http.Client {
	if listenerURL.Host == "" {
		listenerURL.Host = "127.0.0.1"
	}

	var addr string
	switch listenerURL.Scheme {
	case "unix":
		addr = listenerURL.Path
	default:
		addr = listenerURL.Host
	}

	return &http.Client{
		Transport: &http.Transport{
			Dial: func(proto, _ string) (conn net.Conn, err error) {
				return net.Dial(listenerURL.Scheme, addr)
			},
		},
	}
}

// NewClient returns a client that can make HTTP calls over unix or tcp transports
func NewClient(addr string) (*Client, error) {
	listenerURL, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	httpClient := newHTTPClient(listenerURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		endpoint: listenerURL,
		c:        httpClient,
	}, nil
}

// GetHTTPClient returns the http client
func (d *Client) GetHTTPClient() *http.Client {
	return d.c
}

// GetEndpoint returns a copy of the endpoint for this client.
func (d *Client) GetEndpoint() *url.URL {
	copy := *d.endpoint
	return &copy
}

// Call implements the Callable interface.  Makes a call to a supported endpoint.
func (d *Client) Call(endpoint plugin.Endpoint, message, result interface{}) ([]byte, error) {

	// We only support http here.
	ep, err := GetHTTPEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	m := strings.ToUpper(ep.Method)
	url := fmt.Sprintf("http://%s%s", d.endpoint.Host, ep.Path)

	var payload io.Reader
	if message != nil {
		if buff, err := json.Marshal(message); err == nil {
			payload = bytes.NewBuffer(buff)
		} else {
			return nil, err
		}
	}

	tee := new(bytes.Buffer)
	request, err := http.NewRequest(m, url, io.TeeReader(payload, tee))
	if err != nil {
		return nil, err
	}
	resp, err := d.c.Do(request)

	logrus.Debugln("REQ --", d.endpoint.String(), "url=", url, "method=", m, "request=", string(tee.Bytes()), "err=", err)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	buff, err := ioutil.ReadAll(resp.Body)

	logrus.Debugln("RESP -", d.endpoint.String(), "url=", url, "method=", m, "response=", string(buff), "err=", err)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error:%d, msg=%s", resp.StatusCode, string(buff))
	}

	if result != nil {
		err = json.Unmarshal(buff, result)
	}

	return buff, err
}
