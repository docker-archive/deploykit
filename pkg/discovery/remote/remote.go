package remote

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"

	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "discovery/remote")

// NewPluginDiscovery creates plugin lookup given a list of urls
func NewPluginDiscovery(remotes []*url.URL) (discovery.Plugins, error) {
	d := &remotePluginDiscovery{
		remotes: remotes,
	}

	_, err := d.List()
	return d, err
}

type remotePluginDiscovery struct {
	remotes []*url.URL
	lock    sync.Mutex
}

// List returns a list of plugins known, keyed by the name
func (r *remotePluginDiscovery) List() (map[string]*plugin.Endpoint, error) {

	r.lock.Lock()
	defer r.lock.Unlock()

	plugins := map[string]*plugin.Endpoint{}

	for _, remote := range r.remotes {

		// for each remote, we issue the OPTIONS call to get information about
		// master, plugins
		c := &http.Client{}

		body, err := doHTTPOptions(remote, nil, c)

		log.Debug("http options", "remote", remote, "body", string(body), "err", err)

		if err != nil {
			return nil, err
		}

		list := []string{}
		if err := types.AnyBytes(body).Decode(&list); err != nil {
			return nil, err
		}

		for _, p := range list {
			plugins[p] = &plugin.Endpoint{
				Name: p,
				// Protocol is the transport protocol -- unix, tcp, etc.
				Protocol: remote.Scheme,
				// Address is the how to connect - socket file, host:port, etc.
				Address: remote.Host,
			}
		}
	}

	return plugins, nil
}

// Find returns a plugin by name
func (r *remotePluginDiscovery) Find(name plugin.Name) (*plugin.Endpoint, error) {
	lookup, _ := name.GetLookupAndType()
	plugins, err := r.List()
	if err != nil {
		return nil, err
	}
	p, exists := plugins[lookup]
	if !exists {
		return nil, fmt.Errorf("Plugin not found: %s (looked up using %s)", name, lookup)
	}
	return p, nil
}

func doHTTPOptions(u *url.URL, customize func(*http.Request), client *http.Client) ([]byte, error) {
	req, err := http.NewRequest(http.MethodOptions, u.String(), nil)
	if err != nil {
		return nil, err
	}

	if customize != nil {
		customize(req)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
