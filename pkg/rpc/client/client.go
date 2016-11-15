package client

import (
	"bytes"
	"github.com/gorilla/rpc/v2/json"
	"net"
	"net/http"
)

// Client is an HTTP client for sending JSON-RPC requests.
type Client struct {
	http http.Client
}

// New creates a new Client that communicates with a unix socke.
func New(socketPath string) Client {
	dialUnix := func(proto, addr string) (conn net.Conn, err error) {
		return net.Dial("unix", socketPath)
	}

	return Client{http: http.Client{Transport: &http.Transport{Dial: dialUnix}}}
}

// Call sends an RPC with a method and argument.  The result must be a pointer to the response object.
func (c Client) Call(method string, arg interface{}, result interface{}) error {
	message, err := json.EncodeClientRequest(method, arg)
	if err != nil {
		return err
	}

	resp, err := c.http.Post("http://d/rpc", "application/json", bytes.NewReader(message))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.DecodeClientResponse(resp.Body, result)
}
