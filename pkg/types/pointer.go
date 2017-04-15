package types

import (
	"fmt"
	"sync"
)

// PointerFromPath creates a pointer from the path
func PointerFromPath(p Path) *Pointer {
	return &Pointer{
		path: p, // copy
	}
}

// PointerFromString constructs a pointer from a string path.
func PointerFromString(s string) *Pointer {
	return PointerFromPath(RFC6901ToPath(s).Clean())
}

// Pointer is a JSON pointer where the path is specified per IETF RFC6901  -- see https://tools.ietf.org/html/rfc6901
type Pointer struct {
	path Path
	lock sync.Mutex
}

// Get retrieves the value at the given point path location
func (p *Pointer) Get(v interface{}) interface{} {
	p.lock.Lock()
	defer p.lock.Unlock()

	fmt.Println(">>>>>>", p.path, "v=", v)
	return Get(p.path, v)
}

// Set is a non-copy mutation on the input doc, setting the attribute at the pointer to v
func (p *Pointer) Set(doc, v interface{}) (updated interface{}, err error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return v, fmt.Errorf("not implemented")
}

// MarshalJSON returns the json representation
func (p Pointer) MarshalJSON() ([]byte, error) {
	return p.path.MarshalJSON()
}

// UnmarshalJSON unmarshals the buffer to this struct
func (p *Pointer) UnmarshalJSON(buff []byte) error {
	return (&p.path).UnmarshalJSON(buff)
}
