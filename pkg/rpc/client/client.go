package client

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httputil"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/rpc/v2/json2"
)

// Client is an HTTP client for sending JSON-RPC requests.
type Client struct {
	http http.Client
}

// New creates a new Client that communicates over a unix socket.
func New(socketPath string) Client {
	dialUnix := func(proto, addr string) (conn net.Conn, err error) {
		return net.Dial("unix", socketPath)
	}

	return Client{http: http.Client{Transport: &http.Transport{Dial: dialUnix}}}
}

// Call sends an RPC with a method and argument.  The result must be a pointer to the response object.
func (c Client) Call(method string, arg interface{}, result interface{}) error {
	message, err := json2.EncodeClientRequest(method, arg)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "http://a/", bytes.NewReader(message))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	requestData, err := httputil.DumpRequest(req, true)
	if err == nil {
		log.Debugf("Sending request %s", string(requestData))
	} else {
		log.Error(err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	responseData, err := httputil.DumpResponse(resp, true)
	if err == nil {
		log.Debugf("Received response %s", string(responseData))
	} else {
		log.Error(err)
	}

	return json2.DecodeClientResponse(resp.Body, result)
}
