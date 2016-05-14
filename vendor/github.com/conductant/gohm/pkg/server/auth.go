package server

import (
	"fmt"
	"github.com/conductant/gohm/pkg/auth"
	"golang.org/x/net/context"
	"io/ioutil"
	"net/http"
	"time"
)

func ReadPublicKeyPemFile(filename string) func() []byte {
	return func() []byte {
		bytes, err := ioutil.ReadFile(filename)
		if err != nil {
			panic(fmt.Errorf("Error reading public key pem file %s: %s", filename, err.Error()))
		}
		return bytes
	}
}

func DisableAuth() AuthManager {
	a := Auth{IsAuthOnFunc: AuthOff}
	return a.Init()
}

var (
	AuthOff = func() bool { return false }
	AuthOn  = func() bool { return true }
)

func (data Auth) Init() AuthManager {
	var s Auth = data
	if s.IsAuthOnFunc == nil {
		s.IsAuthOnFunc = func() bool { return true }
	}
	if s.VerifyKeyFunc == nil && s.IsAuthOnFunc() {
		panic(fmt.Errorf("Public key file input function not set."))
	}
	if s.GetTimeFunc == nil {
		s.GetTimeFunc = time.Now
	}
	if s.ErrorRenderFunc == nil {
		s.ErrorRenderFunc = DefaultErrorRenderer
	}
	if s.InterceptAuthFunc == nil {
		s.InterceptAuthFunc = func(a bool, ctx context.Context) (bool, context.Context) {
			return a, ctx
		}
	}
	return &s
}

func (this *Auth) IsAuthOn() bool {
	return this.IsAuthOnFunc()
}

func (this *Auth) IsAuthorized(scope AuthScope, req *http.Request) (bool, *auth.Token, error) {
	authed := false
	token, err := auth.TokenFromHttp(req, this.VerifyKeyFunc, this.GetTimeFunc)
	switch err {
	case nil:
	case auth.ErrNoPublicKeyFunc, auth.ErrNoAuthToken:
		return !this.IsAuthOn() || scope == AuthScopeNone, nil, ErrNoAuthToken
	default:
		return false, nil, err
	}
	if scope == AuthScopeNone {
		return true, token, nil
	} else {
		authed = token.HasKey(string(scope))
		return authed, token, nil
	}
}

func (this *Auth) interceptAuth(authed bool, ctx context.Context) (bool, context.Context) {
	return this.InterceptAuthFunc(authed, ctx)
}

func (this *Auth) renderError(resp http.ResponseWriter, req *http.Request, message string, code int) {
	this.ErrorRenderFunc(resp, req, message, code)
}
