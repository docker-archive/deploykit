package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller/util"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"strings"
)

// Client is the client that can access the driver either via tcp or unix
type Client struct {
	UnixSocket string
	c          *http.Client
	Host       string
}

// NewClient creates a client to the controller
func NewClient(socket string) *Client {
	client := &http.Client{}

	host := filepath.Base(socket) // for host name in case it's tcp
	index := strings.Index(host, ":")
	if index == 0 {
		// e.g. :9090 ==> change to localhost:9090, otherwise take the host:port as is.
		host = "localhost" + host
	}

	if util.ProtocolFromListenString(socket) == "unix" {
		client.Transport = &http.Transport{
			Dial: func(proto, addr string) (conn net.Conn, err error) {
				return net.Dial("unix", socket)
			},
		}
		host = "local" // dummy host for unix socket
	}
	return &Client{
		UnixSocket: socket,
		c:          client,
		Host:       host,
	}
}

// Call makes a POST call of the form of /v1/{op}.  For example  /v1/scaler.Start
func (d *Client) Call(op string, req interface{}) error {
	buff, err := json.Marshal(req)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s/v1/%s", d.Host, op)
	resp, err := d.c.Post(url, "application/json", bytes.NewBuffer(buff))
	if err != nil {
		return err
	}
	buff, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	log.Infoln("Resp", string(buff))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error from controller:%d, msg=%s", resp.StatusCode, string(buff))
	}
	return nil
}
