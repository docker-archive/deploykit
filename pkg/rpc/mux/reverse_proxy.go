package mux

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	log "github.com/golang/glog"
)

// ReverseProxy is the mux reverse proxy that is able to multiplex calls to this to different
// backends, including socket-based plugins.
type ReverseProxy struct {
	http.Handler
	plugins      func() discovery.Plugins
	errorHandler func(http.ResponseWriter, *http.Request, string, int)
}

// NewReverseProxy creates a mux reverse proxy
func NewReverseProxy(plugins func() discovery.Plugins) *ReverseProxy {
	rp := &ReverseProxy{
		plugins:      plugins,
		errorHandler: defaultErrorRenderer,
	}
	return rp
}

// ServeHTTP implements HTTP handler
func (rp *ReverseProxy) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
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
	// TODO - set up a default backend that will serve the request??
	rp.errorHandler(resp, req, "cannot resolve handler", http.StatusInternalServerError)
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

func defaultErrorRenderer(resp http.ResponseWriter, req *http.Request, message string, code int) {
	resp.WriteHeader(code)
	escaped := message
	if len(message) > 0 {
		escaped = strings.Replace(message, "\"", "'", -1)
	}
	resp.Write([]byte(fmt.Sprintf("{ \"error\": \"%s\" }", escaped)))
}
