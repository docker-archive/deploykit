package client

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	//	"os"
	"path"
	"sync"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/gorilla/rpc/v2/json2"
)

var (
	log    = logutil.New("module", "rpc/client")
	debugV = logutil.V(1100)
)

type client struct {
	http *http.Client
	addr string
	url  *url.URL
}

// NewHandshaker returns a handshaker object, or a generic, untyped rpc object
func NewHandshaker(address string) (rpc.Handshaker, error) {
	u, httpC, err := parseAddress(address)
	if err != nil {
		return nil, err
	}
	return &client{addr: address, http: httpC, url: u}, nil
}

// FromHandshaker returns a full rpc client from handshaker
func FromHandshaker(handshaker rpc.Handshaker, api spi.InterfaceSpec) (Client, error) {
	unvalidatedClient, is := handshaker.(*client)
	if !is {
		return nil, fmt.Errorf("not a valid client")
	}
	cl := &handshakingClient{client: unvalidatedClient, iface: api, lock: &sync.Mutex{}}
	// check handshake
	if err := cl.handshake(); err != nil {
		// Note - we still return the client with the possibility of doing a handshake later on
		// if we provide an api for the plugin to recheck later.  This way, individual components
		// can stay running and recalibrate themselves after the user has corrected the problems.
		return cl, err
	}
	return cl, nil
}

// New creates a new Client that communicates with a unix socket and validates the remote API.
func New(address string, api spi.InterfaceSpec) (Client, error) {
	u, httpC, err := parseAddress(address)
	if err != nil {
		return nil, err
	}
	unvalidatedClient := &client{addr: address, http: httpC, url: u}
	cl := &handshakingClient{client: unvalidatedClient, iface: api, lock: &sync.Mutex{}}
	// check handshake
	if err := cl.handshake(); err != nil {
		// Note - we still return the client with the possibility of doing a handshake later on
		// if we provide an api for the plugin to recheck later.  This way, individual components
		// can stay running and recalibrate themselves after the user has corrected the problems.
		return cl, err
	}
	return cl, nil
}

func parseAddress(address string) (*url.URL, *http.Client, error) {
	if path.Ext(address) == ".listen" {
		buff, err := ioutil.ReadFile(address)
		if err != nil {
			return nil, nil, err
		}
		address = string(buff)
	}

	u, err := url.Parse(address)
	if err != nil {
		return nil, nil, err
	}
	switch u.Scheme {
	case "", "unix", "file":
		// Socket case
		u.Scheme = "http"
		u.Host = "h"
		u.Path = "" // clear it since it's a file path and we are using it to connect.
		return u, &http.Client{
			Timeout: local.ClientTimeout(),
			Transport: &http.Transport{
				// TODO(chungers) - fix this deprecation
				Dial: func(proto, addr string) (conn net.Conn, err error) {
					return net.Dial("unix", address)
				},
				MaxIdleConns:          1,
				IdleConnTimeout:       1 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			}}, nil
	case "tcp":
		u.Scheme = "http"
		fallthrough
	case "http", "https":
		transport := &http.Transport{
			Dial: (&net.Dialer{
				Timeout: local.ClientTimeout(),
			}).Dial,
			TLSHandshakeTimeout: local.ClientTimeout(),
		}
		return u, &http.Client{Transport: transport}, nil

	default:
	}
	return nil, nil, fmt.Errorf("invalid address %v", address)
}

// Hello implements the Handshaker interface
func (c client) Hello() (map[spi.InterfaceSpec][]rpc.Object, error) {
	req := rpc.HelloRequest{}
	resp := rpc.HelloResponse{}
	if err := c.Call("Handshake.Hello", req, &resp); err != nil {
		return nil, err
	}
	objects := map[spi.InterfaceSpec][]rpc.Object{}
	for k, o := range resp.Objects {
		objects[spi.DecodeInterfaceSpec(k)] = o
	}
	return objects, nil
}

func (c client) Addr() string {
	return c.addr
}

func (c client) Call(method string, arg interface{}, result interface{}) error {
	message, err := json2.EncodeClientRequest(method, arg)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.url.String(), bytes.NewReader(message))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	requestData, err := httputil.DumpRequest(req, true)
	if err == nil {
		log.Debug("Client SEND", "addr", c.addr, "payload", string(requestData), "V", debugV)
	} else {
		log.Warn("Client SEND", "addr", c.addr, "err", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	responseData, err := httputil.DumpResponse(resp, true)
	if err == nil {
		log.Debug("Client RECEIVE", "addr", c.addr, "payload", string(responseData), "V", debugV)
	} else {
		log.Warn("Client RECEIVE", "addr", c.addr, "err", err)
	}

	return json2.DecodeClientResponse(resp.Body, result)
}
