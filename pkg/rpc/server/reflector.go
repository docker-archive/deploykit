package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/docker/infrakit/pkg/plugin"
)

const (
	nullTypeVersion = plugin.TypeVersion("")
)

var (
	// Precompute the reflect.Type of error and http.Request -- from gorilla/rpc
	typeOfError       = reflect.TypeOf((*error)(nil)).Elem()
	typeOfHTTPRequest = reflect.TypeOf((*http.Request)(nil)).Elem()
)

type reflector struct {
	target interface{}
}

func (r *reflector) Info() plugin.Info {
	if i, is := r.target.(plugin.Vendor); is {
		return i.Info()
	}
	return plugin.NoInfo
}

func (r *reflector) exampleProperties() *json.RawMessage {
	if example, is := r.target.(plugin.InputExample); is {
		return example.ExampleProperties()
	}
	return nil
}

// Given the input value, look for all the fields named 'fn' that are typed
// *json.RawMessage and set the field with the example value from the plugin (vendor
// implementation).  This will recurse through all the nested structures.  Fields with
// nil pointers of complex types will be set to a zero value.  The input val parameter
// should be a pointer so that actual struct fields are mutated as needed.
func setFieldValue(fn string, val reflect.Value, example *json.RawMessage, recurse bool) {

	v := reflect.Indirect(val)
	if v.Type().Kind() != reflect.Struct {
		return
	}

	p := v.FieldByName(fn)
	if p.IsValid() && p.Type() == reflect.TypeOf(&json.RawMessage{}) {
		if p.IsNil() && p.CanSet() {
			p.Set(reflect.ValueOf(example))
		}
	}

	if !recurse {
		return
	}

	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		switch f.Type().Kind() {
		case reflect.Struct:
			if f.CanAddr() {
				setFieldValue(fn, f.Addr(), example, true)
			}
		case reflect.Ptr:
			if f.Type().Elem().Kind() == reflect.Struct {
				var c reflect.Value
				if f.IsNil() {
					c = reflect.New(f.Type().Elem())
				} else {
					c = f
				}
				f.Set(c)
				setFieldValue(fn, c, example, true)
			}
		}
	}
}

// Type returns the target's type, taking into account of pointer receiver
func (r *reflector) targetType() reflect.Type {
	return reflect.Indirect(reflect.ValueOf(r.target)).Type()
}

func (r *reflector) validate() error {
	t := r.targetType()
	if !strings.Contains(t.PkgPath(), "github.com/docker/infrakit/pkg/rpc") {
		return fmt.Errorf("object not a standard plugin type: %v", t)
	}
	return nil
}

// TypeVersion returns the plugin type and version.  The plugin rpc object is verified and
// the current version in semver is concatenated with '/' as separator.
func (r *reflector) TypeVersion() (plugin.TypeVersion, error) {
	return plugin.TypeVersion(fmt.Sprintf("%s/%s", r.getPluginTypeName(), plugin.CurrentVersion)), nil
}

// isExported returns true of a string is an exported (upper case) name. -- from gorilla/rpc
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// isExportedOrBuiltin returns true if a type is exported or a builtin -- from gorilla/rpc
func isExportedOrBuiltin(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

func (r *reflector) getPluginTypeName() string {
	return r.targetType().Name()
}

func (r *reflector) toDescription(m reflect.Method) plugin.MethodDescription {
	method := fmt.Sprintf("%s.%s", r.getPluginTypeName(), m.Name)
	input := reflect.New(m.Type.In(2).Elem())
	d := plugin.MethodDescription{
		Method: method,
		// JSON-RPC 1.0 wrapper calls for an array for params
		Params: []interface{}{
			input.Interface(),
		},
		Result: reflect.Zero(m.Type.In(3).Elem()).Interface(),
	}
	return d
}

// pluginMethods returns a slice of methods that match the criteria for exporting as RPC service
func (r *reflector) pluginMethods() []reflect.Method {
	matches := []reflect.Method{}
	receiverT := reflect.TypeOf(r.target)
	for i := 0; i < receiverT.NumMethod(); i++ {

		method := receiverT.Method(i)
		mtype := method.Type

		// Code from gorilla/rpc
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs four ins: receiver, *http.Request, *args, *reply.
		if mtype.NumIn() != 4 {
			continue
		}
		// First argument must be a pointer and must be http.Request.
		reqType := mtype.In(1)
		if reqType.Kind() != reflect.Ptr || reqType.Elem() != typeOfHTTPRequest {
			continue
		}
		// Second argument must be a pointer and must be exported.
		args := mtype.In(2)
		if args.Kind() != reflect.Ptr || !isExportedOrBuiltin(args) {
			continue
		}
		// Third argument must be a pointer and must be exported.
		reply := mtype.In(3)
		if reply.Kind() != reflect.Ptr || !isExportedOrBuiltin(reply) {
			continue
		}
		// Method needs one out: error.
		if mtype.NumOut() != 1 {
			continue
		}
		if returnType := mtype.Out(0); returnType != typeOfError {
			continue
		}

		matches = append(matches, method)
	}
	return matches
}
