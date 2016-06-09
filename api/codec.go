package api

import (
	"encoding/json"
	"net/http"
)

// Codec puts supported format / encoding for objects in one place.
type Codec struct {
	ContentType string
	marshal     func(v interface{}) ([]byte, error)
	unmarshal   func(data []byte, v interface{}) error
}

var (
	// ContentTypeJSON is the codec for JSON
	ContentTypeJSON = &Codec{
		ContentType: "application/json",
		marshal: func(v interface{}) ([]byte, error) {
			return json.Marshal(v)
		},
		unmarshal: func(data []byte, v interface{}) error {
			return json.Unmarshal(data, v)
		},
	}
	// DefaultContentType is the content type assumed when
	// user does not specify the content type is http calls or in api calls
	// with nil Codec
	DefaultContentType = ContentTypeJSON
)

// Respond sets the content type of the response and writes the encoded bytes.
func (c *Codec) Respond(resp http.ResponseWriter, val interface{}) {
	buff, err := c.marshal(val)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	resp.Header().Set("Content-Type", c.ContentType)
	resp.Write(buff)
}
