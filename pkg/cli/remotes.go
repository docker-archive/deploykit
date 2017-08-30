package cli

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/ssh"
)

const (
	// EnvInfrakitHost is the environment variable to set to point to specific backends.
	// The value is used as key into the $INFRAKIT_HOME/hosts file.
	EnvInfrakitHost = "INFRAKIT_HOST"
	// EnvHostsFile is the location of the hosts file
	EnvHostsFile = "INFRAKIT_HOSTS_FILE"
)

// HostsFile returns the hsots file used for looking up hosts
func HostsFile() string {
	return local.Getenv(EnvHostsFile, filepath.Join(local.InfrakitHome(), "hosts"))
}

// Remote is a remote infrakit endpoint
type Remote struct {
	Endpoints HostList
	SSH       string // The bastion host
	User      string
}

// parse the , delimited string into a url list
func (r Remote) endpoints() ([]*url.URL, error) {
	ulist := []*url.URL{}
	for _, h := range strings.Split(string(r.Endpoints), ",") {
		addProtocol := false
		if !strings.Contains(h, "://") {
			h = "http://" + h
			addProtocol = true
		}
		u, err := url.Parse(h)
		if err != nil {
			return nil, err
		}
		if addProtocol {
			u.Scheme = "http"
		}
		ulist = append(ulist, u)
	}
	return ulist, nil
}

// HostList is a comma-delimited list of protocol://host:port
type HostList string

// Hosts is the schema of the hosts file
type Hosts map[string]Remote

// Save saves the hosts
func (h Hosts) Save() error {
	any, err := types.AnyValue(h)
	if err != nil {
		return err
	}
	buff, err := any.MarshalYAML()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(HostsFile(), buff, 0755)
}

// LoadHosts loads the hosts file
func LoadHosts() (Hosts, error) {
	hosts := Hosts{}

	buff, err := ioutil.ReadFile(HostsFile())
	if err != nil {
		if !os.IsExist(err) {
			return hosts, nil
		}
		return nil, err
	}

	any, err := types.AnyYAML(buff)
	if err != nil {
		any = types.AnyBytes(buff)
	}

	return hosts, any.Decode(&hosts)
}

// Remotes returns a list of remote URLs to connect to
func Remotes() ([]*url.URL, error) {
	ulist := []*url.URL{}

	// See if INFRAKIT_HOST is set to point to a host list in the $INFRAKIT_HOME/hosts file.
	host := os.Getenv(EnvInfrakitHost)
	if host == "" {
		return ulist, nil // do nothing -- local mode
	}

	// If the env is set but we don't have any hosts file locally, don't exit.
	// Print a warning and proceed.
	// Now look up the host lists in the file
	hostsFile := HostsFile()
	buff, err := ioutil.ReadFile(hostsFile)
	if err != nil {
		return ulist, nil // do nothing -- local mode
	}

	m := Hosts{}
	yaml, err := types.AnyYAML(buff)
	if err != nil {
		return nil, fmt.Errorf("bad format for hosts file at %s for INFRAKIT_HOST=%s, err=%v", hostsFile, host, err)
	}
	err = yaml.Decode(&m)
	if err != nil {
		return nil, fmt.Errorf("cannot decode hosts file at %s for INFRAKIT_HOST=%s, err=%v", hostsFile, host, err)
	}

	remote, has := m[host]
	if !has {
		return nil, fmt.Errorf("no entry in hosts file at %s for INFRAKIT_HOST=%s", hostsFile, host)
	}

	if remote.SSH != "" {
		return urlFromTunnel(host, remote)
	}
	return remote.endpoints()
}

var (
	tunnels     = map[string]*site{}
	tunnelsLock = sync.Mutex{}
)

type site struct {
	rule    Remote
	urls    []*url.URL
	tunnels []*ssh.Tunnel
}

func (r *site) startTunnels() ([]*url.URL, error) {
	endpoints, err := r.rule.endpoints()
	if err != nil {
		return nil, err
	}

	// endpoints are urls.  url.Host is host:port.
	for _, u := range endpoints {

		host, port, err := net.SplitHostPort(u.Host)
		if err != nil {
			return nil, err
		}

		bastionHost := r.rule.SSH
		if bastionHost == "" {
			bastionHost = u.Host
		}
		if h, p, err := net.SplitHostPort(bastionHost); err == nil && p == "" {
			bastionHost = net.JoinHostPort(h, "22") // default
		}

		config := ssh.DefaultClientConfig()
		config.User = r.rule.User
		tunnel := &ssh.Tunnel{
			Local:  ssh.HostPort(fmt.Sprintf("%s:%d", "127.0.0.1", ssh.RandPort(2200, 2299))),
			Server: ssh.HostPort(bastionHost),
			Remote: ssh.HostPort(net.JoinHostPort(host, port)),
			Config: &config,
		}

		err = tunnel.Start()
		if err != nil {
			return nil, err
		}

		r.tunnels = append(r.tunnels, tunnel)
		r.urls = append(r.urls, &url.URL{
			Scheme: "http",
			Host:   string(tunnel.Local),
		})
	}
	return r.urls, nil
}

func urlFromTunnel(host string, remote Remote) ([]*url.URL, error) {
	tunnelsLock.Lock()
	defer tunnelsLock.Unlock()

	t, has := tunnels[host]
	if has {
		return t.urls, nil
	}
	tunnels[host] = &site{
		rule:    remote,
		urls:    []*url.URL{},
		tunnels: []*ssh.Tunnel{},
	}
	if remote.SSH != "" {
		return tunnels[host].startTunnels()
	}
	return nil, nil
}
