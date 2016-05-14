package server

import (
	"bytes"
	"github.com/conductant/gohm/pkg/encoding"
	"net/http"
)

func Unmarshal(resp http.ResponseWriter, req *http.Request, value interface{}) error {
	defer req.Body.Close()
	contentType := ContentTypeForRequest(req)
	if t, err := encoding.ContentTypeFromString(contentType); err == nil {
		err := encoding.Unmarshal(t, req.Body, value)
		if err != nil {
			DefaultErrorRenderer(resp, req, err.Error(), http.StatusInternalServerError)
			return err
		}
	} else {
		DefaultErrorRenderer(resp, req, ErrBadContentType.Error(), http.StatusBadRequest)
		return err
	}
	return nil
}

func Marshal(resp http.ResponseWriter, req *http.Request, value interface{}) error {
	contentType := ContentTypeForResponse(req)
	if t, err := encoding.ContentTypeFromString(contentType); err == nil {
		buff := new(bytes.Buffer)
		err := encoding.Marshal(t, buff, value)
		if err != nil {
			DefaultErrorRenderer(resp, req, err.Error(), http.StatusInternalServerError)
			return err
		}
		resp.Header().Add("Content-Type", t.String())
		resp.Write(buff.Bytes())
	} else {
		DefaultErrorRenderer(resp, req, ErrBadContentType.Error(), http.StatusBadRequest)
		return err
	}
	return nil
}

func ContentTypeForRequest(req *http.Request) string {
	t := "application/json"

	if req.Method == "POST" || req.Method == "PUT" {
		t = req.Header.Get("Content-Type")
	}
	switch t {
	case "*/*":
		return encoding.ContentTypeJSON.String()
	case "":
		return encoding.ContentTypeJSON.String()
	default:
		return t
	}
}

func ContentTypeForResponse(req *http.Request) string {
	t := req.Header.Get("Accept")
	switch t {
	case "*/*":
		return encoding.ContentTypeJSON.String()
	case "":
		return ContentTypeForRequest(req) // use the same content type as the request if no accept header
	default:
		return t
	}
}
