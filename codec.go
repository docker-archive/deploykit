package libmachete

import (
	"encoding/json"
	"gopkg.in/yaml.v2"
	"net/http"
)

type Codec struct {
	ContentType string
	marshal     func(v interface{}) ([]byte, error)
	unmarshal   func(data []byte, v interface{}) error
}

var (
	ContentTypeJSON = &Codec{
		ContentType: "application/json",
		marshal: func(v interface{}) ([]byte, error) {
			return json.Marshal(v)
		},
		unmarshal: func(data []byte, v interface{}) error {
			return json.Unmarshal(data, v)
		},
	}

	ContentTypeYAML = &Codec{
		ContentType: "text/plain",
		marshal: func(v interface{}) ([]byte, error) {
			return yaml.Marshal(v)
		},
		unmarshal: func(data []byte, v interface{}) error {
			return yaml.Unmarshal(data, v)
		},
	}

	DefaultContentType = ContentTypeJSON

	codecs = map[string]*Codec{
		ContentTypeJSON.ContentType: ContentTypeJSON,
		ContentTypeYAML.ContentType: ContentTypeYAML,
	}
)

// CodecByContentTypeHeader returns the codec based on what's set in the http header
func CodecByContentTypeHeader(req *http.Request) *Codec {
	return CodecByContentType(req.Header.Get("Content-Type"))
}

// CodecByContentType returns a code by content type string such as HTTP header 'Content-Type'
// The default is JSON if the content type is not a supported one.
func CodecByContentType(t string) *Codec {
	if c, ok := codecs[t]; ok {
		return c
	}
	return DefaultContentType
}

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
