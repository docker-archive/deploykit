package api

import (
	"encoding/json"
)

// Codec is responsible for transforming structs to and from text.
type Codec interface {
	// Marshal converts a struct to text.
	Marshal(v interface{}) ([]byte, error)

	// Unmarshal converts text to a struct.
	Unmarshal(data []byte, v interface{}) error
}

type contentTypeJSON struct {
}

func (c contentTypeJSON) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (c contentTypeJSON) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// ContentTypeJSON is a codec that transforms into and from JSON.
var ContentTypeJSON = contentTypeJSON{}
