package remote

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "discovery/remote")

// ParseURLMust returns a list of urls from string. Panics on any errors
func ParseURLMust(s ...string) []*url.URL {
	out := []*url.URL{}
	for _, ss := range s {

		addProtocol := false
		if !strings.Contains(ss, "://") {
			ss = "http://" + ss
			addProtocol = true
		}

		u, err := url.Parse(ss)
		if err != nil {
			panic(err)
		}
		if addProtocol {
			u.Scheme = "http"
		}
		out = append(out, u)
	}
	return out
}

// NewPluginDiscovery creates plugin lookup given a list of urls
func NewPluginDiscovery(remotes []*url.URL) (discovery.Plugins, error) {
	d := &remotePluginDiscovery{
		remotes: remotes,
	}

	_, err := d.List()
	return d, err
}

// DiscoveryResponse captures information about the remote node/proxy
type DiscoveryResponse struct {

	// Leader indicates if this node responding is a leader
	Leader bool

	// Plugins is a slice of plugin names on that node
	Plugins []string
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

		// List of plugins and leadership information is available via HTTP OPTIONS call
		body, err := doHTTPOptions(remote, nil, c)

		// If an error occurs, we continue -- with the assumption that at some point
		// one of the remotes which responded would be a leader.  In that case, we
		// don't care about the other ones that failed to respond.
		if err != nil {
			fmt.Println("error getting remote plugin info", "err", err)
			continue
		}

		data := DiscoveryResponse{}
		if err := types.AnyBytes(body).Decode(&data); err != nil {
			return nil, err
		}
		// Only if it's the leader or if it's the only remote do we actually include the endpoints
		// This way, we ensure the proxy has only one set of plugins (those on the leader) and not the replicas.
		if data.Leader || len(r.remotes) == 1 {

			for _, p := range data.Plugins {

				copy := *remote
				if p[len(p)-1] == '/' {
					copy.Path = p
				} else {
					copy.Path = p + "/"
				}

				plugins[p] = &plugin.Endpoint{
					Name: p,
					// Protocol is the transport protocol -- unix, tcp, etc.
					Protocol: copy.Scheme,
					// Address is the how to connect - socket file, host:port, URL, etc.
					Address: copy.String(),
				}
			}
			break
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
		return nil, discovery.ErrNotFound(string(name))
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
