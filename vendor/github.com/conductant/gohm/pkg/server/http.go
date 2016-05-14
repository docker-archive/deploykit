package server

import (
	"net/http"
)

func content_type_for_request(req *http.Request) string {
	t := "application/json"

	if req.Method == "POST" || req.Method == "PUT" {
		t = req.Header.Get("Content-Type")
	}
	switch t {
	case "*/*":
		return "application/json"
	case "":
		return "application/json"
	default:
		return t
	}
}

func content_type_for_response(req *http.Request) string {
	t := req.Header.Get("Accept")
	switch t {
	case "*/*":
		return "application/json"
	case "":
		return content_type_for_request(req) // use the same content type as the request if no accept header
	default:
		return t
	}
}
