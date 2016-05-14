package encoding

import (
	"encoding/json"
	"errors"
	"github.com/golang/protobuf/proto"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
)

func UnmarshalString(r io.Reader, v interface{}) error {
	if _, ok := v.(*string); !ok {
		return errors.New("wrong-type-expects-str-ptr")
	}
	if buff, err := ioutil.ReadAll(r); err == nil {
		ptr := v.(*string)
		*ptr = string(buff)
		return nil
	} else {
		return err
	}
}

func UnmarshalYAML(r io.Reader, v interface{}) error {
	buff, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(buff, v)
}

func UnmarshalJSON(r io.Reader, v interface{}) error {
	dec := json.NewDecoder(r)
	return dec.Decode(v)
}

func UnmarshalProtobuf(r io.Reader, any interface{}) error {
	v, ok := any.(proto.Message)
	if !ok {
		return ErrIncompatibleType
	}
	buff, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return proto.Unmarshal(buff, v)
}

var (
	unmarshalers = map[ContentType]func(io.Reader, interface{}) error{
		ContentTypeDefault:  UnmarshalJSON,
		ContentTypeAny:      UnmarshalJSON,
		ContentTypeJSON:     UnmarshalJSON,
		ContentTypeYAML:     UnmarshalYAML,
		ContentTypeProtobuf: UnmarshalProtobuf,
		ContentTypePlain:    UnmarshalString,
		ContentTypeHTML:     nil,
	}
)

func Unmarshal(t ContentType, reader io.Reader, value interface{}) error {
	if u, has := unmarshalers[t]; has {
		return u(reader, value)
	}
	return ErrUnknownContentType
}
