package cli

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/infrakit/pkg/types"
)

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
	hostsFile := filepath.Join(os.Getenv("INFRAKIT_HOME"), "hosts")
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
