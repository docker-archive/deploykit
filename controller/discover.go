package controller

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
	"sync"
)

var (
	registry     *Registry
	registryLock sync.Mutex
)

// Registry is a service for finding out controllers we have access to.
// TODO(chungers) - this is the integration point with the Plugin system in Docker Engine.
type Registry struct {
	drivers    map[string]*Controller
	namespaces map[string]*Controller
	names      map[string]*Controller
}

// Namespaces return a map of namespace to Controller
func (r *Registry) Namespaces() map[string]*Controller {
	return r.namespaces
}

// Names return a map of names to Controller
func (r *Registry) Names() map[string]*Controller {
	return r.names
}

// GetControllerByName returns a controller by name
func (r *Registry) GetControllerByName(name string) *Controller {
	return r.names[name]
}

// ForEachControllerCapable visits all the controllers that are capable of doing a task or for a lifecycle phase.
func (r *Registry) ForEachControllerCapable(cap string, f func(string, *Controller)) {
nextController:
	for ns, info := range r.namespaces {
		for _, c := range info.Capabilities {
			if cap == c {
				f(ns, info)
				continue nextController
			}
		}
	}
}

// NewRegistry creates a registry instance with the given file directory path.  The entries in the directory
// are either unix socket files or a flat file indicating the tcp port.
func NewRegistry(dir string) (*Registry, error) {
	if registry != nil {
		return registry, nil
	}

	registryLock.Lock()
	defer registryLock.Unlock()

	log.Infoln("Opening:", dir)
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	registry = &Registry{
		drivers:    map[string]*Controller{},
		namespaces: map[string]*Controller{},
		names:      map[string]*Controller{},
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			socket := filepath.Join(dir, entry.Name())
			log.Infoln("Found driver at socket=", socket)
			driverClient := NewClient(socket)
			info, err := driverClient.GetInfo()
			if err != nil {
				log.Warningln("Error from driver", err)
				continue
			}
			log.Infoln("driver info=", info)
			registry.drivers[socket] = info
			registry.namespaces[info.Namespace] = info
			registry.names[info.DriverName] = info
		}
	}
	return registry, nil
}

// Controller is a struct that has the metadata and client for accessing a controller.
// First discover by scanning all files in a directory for all the unix sockets
// then connect to each one to introspect and ask for name, namespace, and phases
// supported.  Phases are 'bootstrap', 'running', 'teardown'.
type Controller struct {
	Info
	Client *Client
}

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
	if strings.Index(host, ":") == 0 {
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

// GetInfo returns information about the controller / driver
func (d *Client) GetInfo() (*Controller, error) {
	resp, err := d.c.Get("http://" + d.Host + "/v1/info")
	if err != nil {
		return nil, err
	}
	buff, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	info := Info{}
	err = json.Unmarshal(buff, &info)
	if err != nil {
		return nil, err
	}

	return &Controller{
		Info:   info,
		Client: d,
	}, nil
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
