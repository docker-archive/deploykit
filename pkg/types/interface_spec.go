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

	// Sub is the name of 'subclass' entity that follows the general contract but has a distinguishing name
	Sub string
}

// String implements the stringer for fmt printing
func (i InterfaceSpec) String() string {
	return i.Encode()
}

// Encode encodes a struct form to string
func (i InterfaceSpec) Encode() string {
	if i.Sub == "" {
		return fmt.Sprintf("%s/%s", i.Name, i.Version)
	}
	return fmt.Sprintf("%s/%s/%s", i.Name, i.Version, i.Sub)
}

// DecodeInterfaceSpec takes a string and returns the struct
func DecodeInterfaceSpec(s string) InterfaceSpec {
	p := strings.SplitN(s, "/", 3)
	if len(p) == 1 {
		return InterfaceSpec{Name: s}
	}
	i := InterfaceSpec{
		Name:    p[0],
		Version: p[1],
	}
	if len(p) == 3 {
		i.Sub = p[2]
	}
	return i
}
