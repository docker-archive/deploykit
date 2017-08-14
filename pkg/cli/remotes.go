package cli

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/types"
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
	return run.GetEnv(EnvHostsFile, filepath.Join(run.InfrakitHome(), "hosts"))
}

// Remote is a remote infrakit endpoint
type Remote struct {
	Endpoints HostList
	TunnelSSH bool
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

	hosts := []string{}
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
	if has {
		hosts = strings.Split(string(remote.Endpoints), ",")
	} else {
		return nil, fmt.Errorf("no entry in hosts file at %s for INFRAKIT_HOST=%s", hostsFile, host)
	}

	for _, h := range hosts {
		addProtocol := false
		if !strings.Contains(h, "://") {
			h = "http://" + h
			addProtocol = true
		}
		u, err := url.Parse(h)
		if err != nil {
			panic(err)
		}
		if addProtocol {
			u.Scheme = "http"
		}
		ulist = append(ulist, u)
	}

	if remote.TunnelSSH {
		return urlFromTunnel(host, remote)
	}
	return ulist, nil
}

var (
	tunnels     = map[string]tunnel{}
	tunnelsLock = sync.Mutex{}
)

type tunnel struct {
	remote Remote
	urls   []*url.URL
}

func urlFromTunnel(host string, remote Remote) ([]*url.URL, error) {
	tunnelsLock.Lock()
	defer tunnelsLock.Unlock()

	t, has := tunnels[host]
	if has {
		return t.urls, nil
	}

	return nil, nil
}
