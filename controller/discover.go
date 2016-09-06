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
	dir         string
	sockets     []string // preset discovered connection strings
	controllers []*Controller
	lock        sync.Mutex
}

func (r *Registry) findBy(f func(*Controller) bool) *Controller {
	for _, c := range r.controllers {
		if f(c) {
			return c
		}
	}
	return nil
}

// GetControllerByName returns a controller by name
func (r *Registry) GetControllerByName(name string) *Controller {
	return r.findBy(func(c *Controller) bool {
		return c.Info.Name == name
	})
}

// GetControllerByNamespace returns the controller matching the namespace in the swim config
func (r *Registry) GetControllerByNamespace(namespace string) *Controller {
	return r.findBy(func(c *Controller) bool {
		return c.Info.Namespace == namespace
	})
}

// NewRegistry creates a registry instance with the given file directory path.  The entries in the directory
// are either unix socket files or a flat file indicating the tcp port.
func NewRegistry(dir string) (*Registry, error) {
	registryLock.Lock()
	defer registryLock.Unlock()

	if registry != nil {
		return registry, nil
	}
	registry = &Registry{
		controllers: []*Controller{},
		dir:         dir,
		sockets:     []string{},
	}
	return registry, registry.Refresh()
}

// SetDiscovery forces the discovery using the input array of connection strings
// Each element of the slice is the connection string that is normally discovered via scanning the driver dir.
func (r *Registry) SetDiscovery(sockets []string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.sockets = sockets
}

// Refresh rescans the driver directory to see what drivers are there.
func (r *Registry) Refresh() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	log.Infoln("Opening:", r.dir)
	entries, err := ioutil.ReadDir(r.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			socket := filepath.Join(r.dir, entry.Name())
			log.Infoln("Found driver at socket=", socket)
			driverClient := NewClient(socket)
			controller, err := driverClient.Controller()
			if err != nil {
				log.Warningln("Error from driver", err)
				continue
			}
			log.Infoln("driver controller=", controller.Info)
			r.controllers = append(r.controllers, controller)
		}
	}

	log.Infoln("Using presets:", r.sockets)
	for _, socket := range r.sockets {
		log.Infoln("Forcing discovery of driver at socket=", socket)
		driverClient := NewClient(socket)
		controller, err := driverClient.Controller()
		if err != nil {
			log.Warningln("Error from driver", err)
			continue
		}
		log.Infoln("driver controller=", controller.Info)
		r.controllers = append(r.controllers, controller)
	}
	return nil
}

// Controller is a struct that has the metadata and client for accessing a controller.
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

// Controller returns a reference to the controller
func (d *Client) Controller() (*Controller, error) {
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

// Info calls the /v1/info endpoint
func (d *Client) Info() (map[string]interface{}, error) {
	url := fmt.Sprintf("http://%s/v1/info", d.Host)
	resp, err := d.c.Get(url)
	if err != nil {
		return nil, err
	}
	buff, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Infoln("Resp", string(buff))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error from controller:%d, msg=%s", resp.StatusCode, string(buff))
	}

	out := map[string]interface{}{}
	err = json.Unmarshal(buff, &out)
	return out, err
}

// Call makes a POST call of the form of /v1/{op}.  For example  /v1/scaler.Start
func (d *Client) Call(op string, req interface{}, out interface{}) error {
	var resp *http.Response
	var err error
	if req != nil {
		buff, err := json.Marshal(req)
		if err != nil {
			return err
		}
		url := fmt.Sprintf("http://%s/v1/%s", d.Host, op)
		log.Infoln("Calling", url, "via POST")
		resp, err = d.c.Post(url, "application/json", bytes.NewBuffer(buff))
		if err != nil {
			return err
		}
	} else {
		url := fmt.Sprintf("http://%s/v1/%s", d.Host, op)
		log.Infoln("Calling", url, "via POST")
		resp, err = d.c.Post(url, "application/json", nil)
		if err != nil {
			return err
		}
	}
	buff, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(buff) > 0 {
		log.Infoln("Resp", string(buff))
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error from controller:%d, msg=%s", resp.StatusCode, string(buff))
		}
		if out != nil {
			return json.Unmarshal(buff, out)
		}
	}
	return nil
}
