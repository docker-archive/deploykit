package testflight

import (
	"io/ioutil"
	"net/http"
)

type Response struct {
	Body        string
	RawBody     []byte
	RawResponse *http.Response
	StatusCode  int
	Header      http.Header
}

func newResponse(response *http.Response) *Response {
	body, _ := ioutil.ReadAll(response.Body)
	return &Response{
		Body:        string(body),
		RawBody:     body,
		RawResponse: response,
		StatusCode:  response.StatusCode,
		Header:      response.Header,
	}
}
