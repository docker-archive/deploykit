package server

import (
	"github.com/conductant/gohm/pkg/auth"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
	"net/http"
	"sync"
)

type methodBinding struct {
	Api     Endpoint
	Handler Handler
}

type engine struct {
	renderError   func(resp http.ResponseWriter, req *http.Request, message string, code int) error
	routes        map[string]*methodBinding // key = route
	functionNames map[string]*methodBinding // key = function name
	router        *mux.Router
	auth          AuthManager
	event_chan    chan *ServerEvent
	done_chan     chan bool
	webhooks      WebhookManager
	sseChannels   map[string]*sseChannel
	lock          sync.Mutex
	running       bool
}

func (this *engine) buildContext(resp http.ResponseWriter, req *http.Request, token *auth.Token) context.Context {
	return &serverContext{
		Context: context.Background(),
		req:     req,
		resp:    resp,
		token:   token,
		engine:  this,
	}
}

// Unified handler
func httpHandler(engine *engine, binding *methodBinding, am AuthManager) func(http.ResponseWriter, *http.Request) {
	return func(resp http.ResponseWriter, req *http.Request) {
		authed, token, err := am.IsAuthorized(binding.Api.AuthScope, req)
		switch err {
		case nil, ErrNoAuthToken: // continue
		default:
			am.renderError(resp, req, "error", http.StatusBadRequest)
			return
		}
		ctx := engine.buildContext(resp, req, token)
		authed, ctx = am.interceptAuth(authed, ctx)
		if authed {
			binding.Handler(ctx, resp, req)
			return
		} else {
			am.renderError(resp, req, "not-permitted", http.StatusUnauthorized)
		}
		return
	}
}

func (this *engine) ServeHTTP(resp http.ResponseWriter, request *http.Request) {
	defer this.lock.Unlock()
	this.lock.Lock()
	if !this.running {
		// Also start listening on the event channel for any webhook calls
		go func() {
			for {
				select {
				case message := <-this.event_chan:
					this.do_callback(message)

				case done := <-this.done_chan:
					if done {
						glog.Infoln("REST engine event channel stopped.")
						return
					}
				}
			}
		}()
		this.running = true
	}
	this.router.ServeHTTP(resp, request)
}

func (this *engine) Handle(path string, handler http.Handler) {
	this.router.Handle(path, handler)
}

func (this *engine) EventChannel() chan<- *ServerEvent {
	return this.event_chan
}

func (this *engine) do_callback(message *ServerEvent) error {
	if this.webhooks == nil {
		return nil
	}
	if m, has := this.routes[message.Route]; has {
		if m.Api.CallbackEvent != EventKey("") {
			return this.webhooks.Send(message.Key, string(m.Api.CallbackEvent),
				message.Body, m.Api.CallbackBodyTemplate)
		}
	}
	return ErrUnknownMethod
}
