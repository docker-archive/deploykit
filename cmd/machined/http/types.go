package http

import (
	"net/http"
)

// Handler is shorthand for an HTTP request handler function.
type Handler func(resp http.ResponseWriter, req *http.Request)
