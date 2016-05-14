package server

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// Simple, single-host reverse proxy.  This assumes that the backend information can be somehow determined and
// that there are some simple url stripping that takes place.  See the test case for example usage to build
// a trivial reverse proxy that can optionally support token authentication.
type ReverseProxy struct {
	http.Handler

	strip string

	scheme string
	host   string
	port   string
	prefix string

	lock       sync.Mutex
	delegate   http.Handler
	errHandler func(http.ResponseWriter, *http.Request, string, int) error
}

func NewReverseProxy() *ReverseProxy {
	return &ReverseProxy{
		errHandler: DefaultErrorRenderer,
		scheme:     "http",
	}
}

func (this *ReverseProxy) SetForwardScheme(s string) *ReverseProxy {
	this.scheme = s
	return this
}

func (this *ReverseProxy) SetForwardHost(h string) *ReverseProxy {
	this.host = h
	return this
}

func (this *ReverseProxy) SetForwardPort(p int) *ReverseProxy {
	this.port = fmt.Sprintf(":%d", p)
	return this
}

func (this *ReverseProxy) SetForwardHostPort(hp string) *ReverseProxy {
	if i := strings.Index(hp, ":"); i > -1 {
		this.host = hp[0:i]
		if len(this.host) == 0 {
			this.host = "127.0.0.1" // no host specified
		}
		this.port = hp[i:]
	} else {
		this.host = hp
	}
	return this
}

func (this *ReverseProxy) SetForwardPrefix(p string) *ReverseProxy {
	this.prefix = p
	return this
}

func (this *ReverseProxy) Strip(s string) *ReverseProxy {
	this.strip = s
	return this
}

func (this *ReverseProxy) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	this.lock.Lock()
	defer this.lock.Unlock()

	if this.delegate == nil {
		urlString := this.reverseProxyUrl()
		u, err := url.Parse(urlString)
		if err != nil {
			this.errHandler(resp, req, urlString, http.StatusInternalServerError)
			return
		}
		this.delegate = http.StripPrefix(this.strip, reverseProxyHandler(u))
	}
	this.delegate.ServeHTTP(resp, req)
}

func (this *ReverseProxy) reverseProxyUrl() string {
	p1 := fmt.Sprintf("%s://%s%s", this.scheme, this.host, this.port)
	if len(this.prefix) > 0 {
		if strings.HasPrefix(this.prefix, "/") {
			return p1 + this.prefix
		} else {
			return p1 + "/" + this.prefix
		}
	}
	return p1
}

func reverseProxyHandler(u *url.URL) http.Handler {
	rp := httputil.NewSingleHostReverseProxy(u)
	// We need to rewrite the request to change the host. This is so that
	// some CDNs that checks for the Host header won't barf.
	// We modify this only after the default Director has done its thing.
	wrapped := rp.Director
	rp.Director = func(req *http.Request) {
		wrapped(req)
		req.Header.Set("Host", u.Host)
		req.Host = u.Host
	}
	return rp
}
