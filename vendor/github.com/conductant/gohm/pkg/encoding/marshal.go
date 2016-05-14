package encoding

import (
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"gopkg.in/yaml.v2"
	"io"
)

func MarshalString(w io.Writer, value interface{}) error {
	switch value := value.(type) {
	case *string:
		w.Write([]byte(*value))
	case string:
		w.Write([]byte(value))
	case []byte:
		w.Write(value)
	default:
		fmt.Fprintf(w, "%v", value)
	}
	return nil
}

func MarshalJSON(w io.Writer, v interface{}) error {
	if buff, err := json.Marshal(v); err == nil {
		w.Write(buff)
		return nil
	} else {
		return err
	}
}

func MarshalYAML(w io.Writer, v interface{}) error {
	if buff, err := yaml.Marshal(v); err == nil {
		w.Write(buff)
		return nil
	} else {
		return err
	}
}

func MarshalProtobuf(w io.Writer, any interface{}) error {
	v, ok := any.(proto.Message)
	if !ok {
		return ErrIncompatibleType
	}
	if buff, err := proto.Marshal(v); err == nil {
		w.Write(buff)
		return nil
	} else {
		return err
	}
}

var (
	marshalers = map[ContentType]func(io.Writer, interface{}) error{
		ContentTypeDefault:  MarshalJSON,
		ContentTypeAny:      MarshalJSON,
		ContentTypeJSON:     MarshalJSON,
		ContentTypeYAML:     MarshalYAML,
		ContentTypeProtobuf: MarshalProtobuf,
		ContentTypePlain:    MarshalString,
		ContentTypeHTML:     nil,
	}
)

// Returns the ContentType given the string.
func ContentTypeFromString(t string) (ContentType, error) {
	if Check(ContentType(t)) {
		return ContentType(t), nil
	}
	return ContentTypeDefault, ErrBadContentType
}

func Check(c ContentType) bool {
	_, canMarshal := marshalers[c]
	_, canUnmarshal := unmarshalers[c]
	return canMarshal && canUnmarshal
}

func Marshal(t ContentType, writer io.Writer, value interface{}) error {
	if m, has := marshalers[t]; has {
		return m(writer, value)
	}
	return ErrUnknownContentType
}
