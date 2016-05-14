package server

import (
	"github.com/conductant/gohm/pkg/auth"
	"golang.org/x/net/context"
	"net/http"
	"time"
)

type Handler func(auth context.Context, resp http.ResponseWriter, req *http.Request)

type ServerEvent struct {
	Timestamp time.Time
	Key       string
	Route     string
	Body      interface{}
}

type Auth struct {
	IsAuthOnFunc      func() bool
	GetTimeFunc       func() time.Time
	VerifyKeyFunc     func() []byte
	ErrorRenderFunc   func(http.ResponseWriter, *http.Request, string, int) error
	InterceptAuthFunc func(bool, context.Context) (bool, context.Context)
}

type AuthManager interface {
	IsAuthOn() bool
	IsAuthorized(AuthScope, *http.Request) (bool, *auth.Token, error)

	interceptAuth(bool, context.Context) (bool, context.Context)
	renderError(http.ResponseWriter, *http.Request, string, int)
}

type Streamer interface {
	EventChannel() chan<- *ServerEvent
	StreamChannel(contentType, eventType, key string) (*sseChannel, bool)
	MergeHttpStream(w http.ResponseWriter, r *http.Request, contentType, eventType, key string, src <-chan interface{}) error
	DirectHttpStream(http.ResponseWriter, *http.Request) (chan<- interface{}, error)
}

type ApiDiscovery interface {
	ApiForScope() Endpoint
	ApiForFunc(func(context.Context, http.ResponseWriter, *http.Request)) Endpoint
}

type Server interface {
	ApiDiscovery
	http.Handler
	Handle(string, http.Handler)
}
