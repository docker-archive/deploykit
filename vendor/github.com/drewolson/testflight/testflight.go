package testflight

import (
	"net/http"
	"net/http/httptest"
)

const (
	JSON         = "application/json"
	FORM_ENCODED = "application/x-www-form-urlencoded"
)

func WithServer(handler http.Handler, context func(*Requester)) {
	server := httptest.NewServer(handler)
	defer server.Close()

	requester := &Requester{server: server}
	context(requester)
}
