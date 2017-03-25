package mux

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	"github.com/docker/infrakit/pkg/discovery"
	manager_discovery "github.com/docker/infrakit/pkg/manager/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
	log "github.com/golang/glog"
)

// ReverseProxy is the mux reverse proxy that is able to multiplex calls to this to different
// backends, including socket-based plugins.
type ReverseProxy struct {
	http.Handler
	plugins func() discovery.Plugins
}

// NewReverseProxy creates a mux reverse proxy
func NewReverseProxy(plugins func() discovery.Plugins) *ReverseProxy {
	rp := &ReverseProxy{
		plugins: plugins,
	}
	return rp
}

func (rp *ReverseProxy) listPlugins(resp http.ResponseWriter, req *http.Request, leader bool) {
	found, err := rp.plugins().List()
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}

	result := []string{}
	for name := range found {
		result = append(result, name)
	}

	data := struct {
		Leader  bool
		Plugins []string
	}{
		Leader:  leader,
		Plugins: result,
	}

	resp.Write(types.AnyValueMust(data).Bytes())
	return
}

// ServeHTTP implements HTTP handler
func (rp *ReverseProxy) ServeHTTP(resp http.ResponseWriter, req *http.Request) {

	if req.URL.Path == "/" {
		switch req.Method {
		case http.MethodOptions:

			leader := false
			if manager, err := manager_discovery.Locate(rp.plugins); err == nil {
				if l, err := manager.IsLeader(); err == nil {
					leader = l
				}
			}
			rp.listPlugins(resp, req, leader)

		default:
			http.NotFound(resp, req)
		}
		return
	}

	p := strings.Split(req.URL.Path, "/")
	if len(p) < 3 {
		http.NotFound(resp, req)
		return
	}

	if p[2] == "events" {
		log.V(100).Infoln("TODO - event stream proxy")

		return
	}

	// standard handling
	handler, prefix := rp.reverseProxyHandler(req.URL)
	if handler != nil {
		if p := strings.TrimPrefix(req.URL.Path, "/"+prefix); len(p) < len(req.URL.Path) {
			req.URL.Path = p
			log.V(100).Infoln("proxying to plugin", prefix, "url=", req.URL.String(), "uri=", req.URL.RequestURI())
			handler.ServeHTTP(resp, req)
		} else {
			http.NotFound(resp, req)
		}
		return
	}

	http.NotFound(resp, req)
	return
}

// returns the handler and the corresponding prefix
func (rp *ReverseProxy) reverseProxyHandler(u *url.URL) (proxy http.Handler, prefix string) {
	var socketPath string

	defer func() {
		log.V(100).Infoln("reverse proxy lookup for url:", u, "socket=", socketPath, "prefix=", prefix)
	}()

	reversep := httputil.NewSingleHostReverseProxy(u)
	socketPath, prefix = rp.socketPath(u)
	if socketPath != "" {
		reversep.Transport = &http.Transport{
			Dial: func(proto, addr string) (conn net.Conn, err error) {
				log.V(100).Infoln("connecting:", proto, socketPath)
				return net.Dial("unix", socketPath)
			},
		}
		u.Scheme = "http"
		u.Host = "d"
	}

	// We need to rewrite the request to change the host. This is so that
	// some CDNs that checks for the Host header won't barf.
	// We modify this only after the default Director has done its thing.
	targetQuery := u.RawQuery
	reversep.Director = func(req *http.Request) {
		req.URL.Scheme = u.Scheme
		req.URL.Host = u.Host
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
		req.Header.Set("Host", u.Host)
		req.Host = u.Host
	}
	proxy = reversep
	return
}

// determines the socketPath based on the request url.  The request url will always be http(s), but
// we need to be able to map some known paths to plugins listening on the localhost's unix sockets.
// Return "" for standard http proxying to another resource.
// The incoming requests look like this https://proxy_ip:proxy_port/plugin_name/...
func (rp *ReverseProxy) socketPath(request *url.URL) (socketPath string, prefix string) {
	name := pluginName(request)
	if name == "" {
		return
	}
	// look for the plugin
	endpoint, err := rp.plugins().Find(plugin.Name(name))
	if err != nil {
		return
	}
	socketPath = endpoint.Address
	prefix = name
	return
}

// pluginName returns the socket name for a url of the form http://ip:port/socket_name/path/to/resource
// it returns "" if path is not specified..
func pluginName(u *url.URL) string {
	if u.Path == "" {
		return ""
	}
	root := ""
	for p := u.Path; p != "/" && p != "."; p = path.Dir(p) {
		root = path.Base(p)
	}
	if root == "." || root == "/" {
		return ""
	}
	return root
}
