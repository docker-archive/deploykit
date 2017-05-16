package cli

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/docker/infrakit/pkg/types"
)

const (
	// HostsFileEnvVar is the location of the hosts file
	HostsFileEnvVar = "INFRAKIT_HOSTS_FILE"
)

// HostsFile returns the hsots file used for looking up hosts
func HostsFile() string {
	if hostsFile := os.Getenv(HostsFileEnvVar); hostsFile != "" {
		return hostsFile
	}

	// if there's INFRAKIT_HOME defined
	home := os.Getenv("INFRAKIT_HOME")
	if home != "" {
		return filepath.Join(home, "hosts")
	}

	home = os.Getenv("HOME")
	if usr, err := user.Current(); err == nil {
		home = usr.HomeDir
	}
	return filepath.Join(home, ".infrakit/hosts")
}

// HostList is a comma-delimited list of protocol://host:port
type HostList string

// Hosts is the schema of the hosts file
type Hosts map[string]HostList

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
	host := os.Getenv("INFRAKIT_HOST")
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

	m := map[string]string{}
	yaml, err := types.AnyYAML(buff)
	if err != nil {
		return nil, fmt.Errorf("bad format for hosts file at %s for INFRAKIT_HOST=%s, err=%v", hostsFile, host, err)
	}
	err = yaml.Decode(&m)
	if err != nil {
		return nil, fmt.Errorf("cannot decode hosts file at %s for INFRAKIT_HOST=%s, err=%v", hostsFile, host, err)
	}

	if list, has := m[host]; has {
		hosts = strings.Split(list, ",")
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

	return ulist, nil
}
