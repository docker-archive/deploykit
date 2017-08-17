package mux

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	manager_discovery "github.com/docker/infrakit/pkg/manager/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/event"
	event_spi "github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/types"
)

// ReverseProxy is the mux reverse proxy that is able to multiplex calls to this to different
// backends, including socket-based plugins.
type ReverseProxy struct {
	http.Handler
	plugins func() discovery.Plugins

	// if this is nil, consider this is local.  If it's not nil, then redirect to this url instead
	forward     *url.URL
	forwardLock sync.Mutex
}

// NewReverseProxy creates a mux reverse proxy
func NewReverseProxy(plugins func() discovery.Plugins) *ReverseProxy {
	rp := &ReverseProxy{
		plugins: plugins,
	}
	return rp
}

// ForwardTo sends the traffic to this url (host:port) instead of local plugins
func (rp *ReverseProxy) ForwardTo(u *url.URL) {
	rp.forwardLock.Lock()
	defer rp.forwardLock.Unlock()
	rp.forward = u
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
	if rp.forward != nil {
		rp.forwardHTTP(resp, req)
		return
	}
	rp.serveHTTPLocal(resp, req)
	return
}

// We need to rewrite the request to change the host. This is so that
// some CDNs that checks for the Host header won't barf.
// We modify this only after the default Director has done its thing.
func defaultDirector(u *url.URL) func(*http.Request) {
	targetQuery := u.RawQuery
	return func(req *http.Request) {
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
}

func (rp *ReverseProxy) forwardHTTP(resp http.ResponseWriter, req *http.Request) {
	log.Debug("forwarding traffic", "url", rp.forward, "V", logutil.V(100), "req", req)
	reversep := httputil.NewSingleHostReverseProxy(rp.forward)
	reversep.Director = defaultDirector(rp.forward)
	handler := &loggingHandler{handler: reversep}
	handler.ServeHTTP(resp, req)
	return
}

// serves HTTP traffic to local resources (on the same node)
func (rp *ReverseProxy) serveHTTPLocal(resp http.ResponseWriter, req *http.Request) {
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

	// URL has the form /plugin_name/
	// There are therefore 3 components
	p := strings.SplitN(req.URL.Path, "/", 3)
	if len(p) < 3 {
		http.NotFound(resp, req)
		return
	}
	// standard handling
	handler, prefix := rp.reverseProxyHandler(req.URL)
	if handler == nil {
		http.NotFound(resp, req)
		return
	}

	handler = &loggingHandler{handler: handler}

	switch p[2] {

	case "events":

		// flusher is required for streaming
		flusher, ok := resp.(http.Flusher)
		if !ok {
			http.Error(resp, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		topic := req.URL.Query().Get("topic")
		log.Info("events", "plugin", prefix, "topic", topic)

		topicPath := types.PathFromString(topic)
		socketPath, _ := rp.socketPath(req.URL)
		if socketPath == "" {
			http.NotFound(resp, req)
			return
		}

		ep, err := event.NewClient(socketPath)
		if err != nil {
			http.Error(resp, "cannot connect to events", http.StatusInternalServerError)
			return
		}
		subscriber, is := ep.(event_spi.Subscriber)
		if !is {
			http.Error(resp, "no subscriber implementation", http.StatusInternalServerError)
			return
		}

		events, stop, err := subscriber.SubscribeOn(topicPath)
		if err != nil {
			http.Error(resp, "cannot conne", http.StatusInternalServerError)
			return
		}

		resp.Header().Set("Content-Type", "text/event-stream")
		resp.Header().Set("Cache-Control", "no-cache")
		resp.Header().Set("Connection", "keep-alive")
		resp.Header().Set("Access-Control-Allow-Origin", "*")

		// Listen to connection close and un-register messageChan
		notify := resp.(http.CloseNotifier).CloseNotify()

		for {
			select {
			case <-notify:
				close(stop)
				log.Debug("disconnected")
				return

			default:

				event := <-events
				buff, err := event.Bytes()
				if err != nil {
					log.Error("proxy error", "topic", topic, "err", err)
					continue
				}

				// Write to the ResponseWriter
				// Server Sent Events compatible
				fmt.Fprintf(resp, "data: %s\n\n", bytes.Replace(buff, []byte{'\n'}, nil, -1))

				// Flush the data immediatly instead of buffering it for later.
				flusher.Flush()

			}
		}

	default:
		target := strings.TrimPrefix(req.URL.Path, "/"+prefix)

		// sanity check -- the target should be shorter
		if len(target) < len(req.URL.Path) {
			req.URL.Path = target
			log.Debug("proxying to plugin", "prefix", prefix, "url", req.URL.String(), "uri", req.URL.RequestURI())
			handler.ServeHTTP(resp, req)
			return
		}
	}

	http.NotFound(resp, req)
	return
}

// returns the handler and the corresponding prefix
func (rp *ReverseProxy) reverseProxyHandler(u *url.URL) (proxy http.Handler, prefix string) {
	var socketPath string

	log.Debug("reverse proxy lookup", "url", u, "socket", socketPath, "prefix", prefix)

	// defer func() {
	// log.Debug("reverse proxy lookup", "url", u, "socket", socketPath, "prefix", prefix)
	// }()

	reversep := httputil.NewSingleHostReverseProxy(u)
	socketPath, prefix = rp.socketPath(u)
	if socketPath != "" {

		uu, err := url.Parse(socketPath)
		if err != nil {
			panic(err) // this should not happen. complain loudly.
		}
		log.Debug("checking socketPath", "socketPath", socketPath, "parsed", uu)

		switch uu.Scheme {
		case "", "unix", "file":
			reversep.Transport = &http.Transport{
				Dial: func(proto, addr string) (conn net.Conn, err error) {
					log.Debug("connecting", "proto", proto, "socket", uu.Path)
					return net.Dial("unix", uu.Path)
				},
			}
			u.Scheme = "http"
			u.Host = "d"

		case "tcp":
			reversep.Transport = &http.Transport{}
			u.Scheme = "http"
			u.Host = uu.Host

		case "http", "https":
			reversep.Transport = &http.Transport{}
			u.Scheme = uu.Scheme
			u.Host = uu.Host

		default:
		}

	}
	reversep.Director = defaultDirector(u)
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
