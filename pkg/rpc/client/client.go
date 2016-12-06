package client

import (
	"bytes"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/gorilla/rpc/v2/json2"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
)

type client struct {
	http http.Client
}

// New creates a new Client that communicates with a unix socket and validates the remote API.
func New(socketPath string, api spi.InterfaceSpec) Client {
	dialUnix := func(proto, addr string) (conn net.Conn, err error) {
		return net.Dial("unix", socketPath)
	}

	unvalidatedClient := &client{http: http.Client{Transport: &http.Transport{Dial: dialUnix}}}
	return &handshakingClient{client: unvalidatedClient, iface: api, lock: &sync.Mutex{}}
}

func (c client) Call(method string, arg interface{}, result interface{}) error {
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
