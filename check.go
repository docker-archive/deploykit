package libmachete

import (
	"fmt"
	"reflect"
	"strings"
)

// FindMissingFields examines the struct provided and based on the field tag annotations, returns a list
// of YAML fields that are required but not provided.
func FindMissingFields(v interface{}) []string {
	missing := []string{}
	CheckFields(v, CollectMissingYAMLFields(&missing))
	return missing
}

// TagGetter is a function that the callback can use to access other tags in the field.
// For example, the callback may want to access the `json` tag in the field to report to
// the user the field name as defined in the tag.
type TagGetter func(tag string) string

// FieldCheckCallback callbacks are called when a struct field fails validation.  Return true to stop/error immediately
type FieldCheckCallback func(value interface{}, fieldName string, getter TagGetter) bool

// CollectMissingYAMLFields returns a callback that will collect all the missing fields into the provided slice.
func CollectMissingYAMLFields(list *[]string) FieldCheckCallback {
	return func(value interface{}, fieldName string, getter TagGetter) bool {
		*list = append(*list, strings.Split(getter("yaml"), ",")[0])
		return false
	}
}

// CheckFields checks the fields in the given struct to see if any of the fields are
// nil or zero value when the field annotation / tag indicates otherwise.  The client
// can use callback to be notified of the event or pass nil for the callbacks and instead
// check the error which contains lists of field names in violation.  The callbacks can return
// false to stop the processing.
func CheckFields(v interface{}, cb FieldCheckCallback) error {
	if reflect.TypeOf(v).Kind() == reflect.Ptr && reflect.TypeOf(v).Elem().Kind() == reflect.Struct {
		err := new(ErrStructFields)
		callbacks := map[string]FieldCheckCallback{
			ruleRequired: ensureNotNil(cb),
		}
		checkFields(v, callbacks, err)
		if len(err.Names) > 0 {
			return err
		}
		return nil
	}
	return fmt.Errorf("Target must be a pointer to struct. Not %v", reflect.TypeOf(v))
}

// ErrStructFields is error used to collect the names of fields that violate the checks.
type ErrStructFields struct {
	Names []string
}

func (e *ErrStructFields) append(v string) {
	e.Names = append(e.Names, v)
}

func (e *ErrStructFields) Error() string {
	return fmt.Sprintf("Invalid entry: missing or zero values in fields:%s",
		strings.Join(e.Names, ","))
}

func ensureNotNil(f FieldCheckCallback) FieldCheckCallback {
	if f == nil {
		return func(interface{}, string, TagGetter) bool {
			return false
		}
	}
	return f
}

const (
	ruleRequired = "required"
)

var (
	// The functions here returns TRUE on failure / violation
	fieldChecks = map[string]func(reflect.Value) bool{
		ruleRequired: func(v reflect.Value) bool {
			switch v.Type().Kind() {
			case reflect.String:
				return violateNotZero(v)
			case reflect.Ptr:
				// special case for *string ==> it's not nil and not zero:
				if v.Type().Elem().Kind() == reflect.String {
					return violateNotNil(v) || violateNotZero(v)
				}
				return violateNotNil(v)
			default:
				return violateNotZero(v)
			}
		},
	}
)

// Returns true if the value violates the not nil rule.
func violateNotNil(v reflect.Value) bool {
	if v.Type().Kind() == reflect.Ptr {
		return v.IsNil()
	}
	return false
}

// Returns true if the value violates the not zero rule
func violateNotZero(v reflect.Value) bool {
	v = reflect.Indirect(v) // if v is pointer, indirect returns *v, otherwise just v
	if v.IsValid() {
		zero := reflect.Zero(v.Type())
		return reflect.DeepEqual(v.Interface(), zero.Interface())
	}
	return false
}

// val should be a pointer
func checkFields(val interface{}, callbacks map[string]FieldCheckCallback, err *ErrStructFields) bool {
	t := reflect.TypeOf(val).Elem()
	v := reflect.Indirect(reflect.ValueOf(val))

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// See https://golang.org/ref/spec#Uniqueness_of_identifiers
		exported := field.PkgPath == ""
		if !exported {
			continue
		}

		switch {
		// Embedded struct
		case field.Anonymous && field.Type.Kind() == reflect.Struct:
			// A struct has been embedded in this struct.
			if checkFields(fieldValue.Addr().Interface(), callbacks, err) {
				return true
			}
			continue
		case field.Type.Kind() == reflect.Struct:
			if checkFields(fieldValue.Addr().Interface(), callbacks, err) {
				return true
			}
			continue
		case field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Struct:
			if checkFields(fieldValue.Interface(), callbacks, err) {
				return true
			}
			continue
		}

		// Normal fields
		tag := field.Tag
		spec := tag.Get("check")
		if spec == "" {
			continue
		}

		rules := strings.Split(spec, ",")
		for _, name := range rules {
			rule, has := fieldChecks[name]
			if !has {
				panic(fmt.Errorf("Programming error: bad rule:%s", name))
			}
			if rule(fieldValue) {
				err.append(field.Name)
				if cb, has := callbacks[name]; has {
					if cb(val, field.Name, func(t string) string { return tag.Get(t) }) {
						return true // true to stop
					}
				}
			}
		}
	}
	return false
}
