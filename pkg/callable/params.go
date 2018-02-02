package callable

import (
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"
)

// Parameters is a struct that implements the backend.Parameters interface.
// It is used for defining the callable's schema and have methods for rendering
// a view of the schema.  It also provides storage of input parameters required
// by a Callable.
type Parameters struct {
	params map[string]param

	lock sync.Mutex
}

type param struct {
	name  string
	value interface{}
	usage string
}

func (p *Parameters) get(name string) (param, bool) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.params == nil {
		p.params = map[string]param{}
	}
	v, has := p.params[name]
	return v, has
}

func (p *Parameters) set(name string, pp param) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.params == nil {
		p.params = map[string]param{}
	}
	p.params[name] = pp
}

// SetParameter sets the value -- TOOD do runtime type checking.  Returns error if not found
// or type mismatch (later).
func (p *Parameters) SetParameter(name string, value interface{}) error {
	param, has := p.get(name)
	if !has {
		return fmt.Errorf("not found %v", name)
	}
	v := value
	reflect.Indirect(reflect.ValueOf(param.value)).Set(reflect.ValueOf(v))
	return nil
}

// StringSlice defines a string slice param
func (p *Parameters) StringSlice(name string, value []string, usage string) *[]string {
	slot := value
	p.set(name, param{name: name, value: &slot, usage: usage})
	return &slot
}

// String defines a string param
func (p *Parameters) String(name string, value string, usage string) *string {
	slot := value
	p.set(name, param{name: name, value: &slot, usage: usage})
	return &slot
}

// Float64 defines a float64 param
func (p *Parameters) Float64(name string, value float64, usage string) *float64 {
	slot := value
	p.set(name, param{name: name, value: &slot, usage: usage})
	return &slot
}

// Int defines an int param
func (p *Parameters) Int(name string, value int, usage string) *int {
	slot := value
	p.set(name, param{name: name, value: &slot, usage: usage})
	return &slot
}

// Bool defines a bool param
func (p *Parameters) Bool(name string, value bool, usage string) *bool {
	slot := value
	p.set(name, param{name: name, value: &slot, usage: usage})
	return &slot
}

// IP defines an IP param
func (p *Parameters) IP(name string, value net.IP, usage string) *net.IP {
	slot := value
	p.set(name, param{name: name, value: &slot, usage: usage})
	return &slot
}

// Duration defines a duration param
func (p *Parameters) Duration(name string, value time.Duration, usage string) *time.Duration {
	slot := value
	p.set(name, param{name: name, value: &slot, usage: usage})
	return &slot
}

// GetStringSlice returns the param by name
func (p *Parameters) GetStringSlice(name string) (v []string, err error) {
	if param, has := p.get(name); has && param.value != nil {
		v = *(param.value.(*[]string))
		return
	}
	err = fmt.Errorf("not found %v", name)
	return
}

// GetString returns the param by name
func (p *Parameters) GetString(name string) (v string, err error) {
	if param, has := p.get(name); has && param.value != nil {
		v = *(param.value.(*string))
		return
	}
	err = fmt.Errorf("not found %v", name)
	return
}

// GetFloat64 returns the param by name
func (p *Parameters) GetFloat64(name string) (v float64, err error) {
	if param, has := p.get(name); has && param.value != nil {
		v = *(param.value.(*float64))
		return
	}
	err = fmt.Errorf("not found %v", name)
	return
}

// GetInt returns the param by name
func (p *Parameters) GetInt(name string) (v int, err error) {
	if param, has := p.get(name); has && param.value != nil {
		v = *(param.value.(*int))
		return
	}
	err = fmt.Errorf("not found %v", name)
	return
}

// GetBool returns the param by name
func (p *Parameters) GetBool(name string) (v bool, err error) {
	if param, has := p.get(name); has && param.value != nil {
		v = *(param.value.(*bool))
		return
	}
	err = fmt.Errorf("not found %v", name)
	return
}

// GetIP returns the param by name
func (p *Parameters) GetIP(name string) (v net.IP, err error) {
	if param, has := p.get(name); has && param.value != nil {
		v = *(param.value.(*net.IP))
		return
	}
	err = fmt.Errorf("not found %v", name)
	return
}

// GetDuration returns the param by name
func (p *Parameters) GetDuration(name string) (v time.Duration, err error) {
	if param, has := p.get(name); has && param.value != nil {
		v = *(param.value.(*time.Duration))
		return
	}
	err = fmt.Errorf("not found %v", name)
	return
}
