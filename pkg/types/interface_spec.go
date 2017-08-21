package types

import (
	"fmt"
	"strings"
)

// InterfaceSpec is metadata about an API.
type InterfaceSpec struct {
	// Name of the interface.
	Name string

	// Version is the identifier for the API version.
	Version string
}

// Encode encodes a struct form to string
func (i InterfaceSpec) Encode() string {
	return fmt.Sprintf("%s/%s", i.Name, i.Version)
}

// DecodeInterfaceSpec takes a string and returns the struct
func DecodeInterfaceSpec(s string) InterfaceSpec {
	p := strings.SplitN(s, "/", 2)
	return InterfaceSpec{
		Name:    p[0],
		Version: p[1],
	}
}
