package template

import (
	"fmt"
	"time"

	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// MetadataFunc returns a template function to support metadata retrieval in templates.
// The function allows an additional parameter to set the value, provided the metadata plugin
// is not readonly (supports the metadata.Updatable spi).  In the case of an update, the returned value
// is always an empty string, with error (can be nil).  The behavior is the same as var function.
func MetadataFunc(scope scope.Scope) func(string, ...interface{}) (interface{}, error) {

	return func(path string, optionalValue ...interface{}) (interface{}, error) {

		switch len(optionalValue) {
		case 0: // read
			return doBlockingGet(scope.Metadata, path, 0, 0)
		case 1: // set
			return doSet(scope.Metadata, path, optionalValue[0])
		case 2: // a retry time + timeout is specified for a read
			retry, err := duration(optionalValue[0])
			if err != nil {
				return nil, err
			}
			timeout, err := duration(optionalValue[1])
			if err != nil {
				return nil, err
			}
			return doBlockingGet(scope.Metadata, path, retry, timeout)
		case 3: // a retry time + timeout is specified for a read + bool to return error
			retry, err := duration(optionalValue[0])
			if err != nil {
				return nil, err
			}
			timeout, err := duration(optionalValue[1])
			if err != nil {
				return nil, err
			}
			errOnTimeout, is := optionalValue[2].(bool)
			if !is {
				return nil, fmt.Errorf("must be boolean %v", optionalValue[2])
			}
			v, err := doBlockingGet(scope.Metadata, path, retry, timeout)
			if errOnTimeout {
				return v, err
			}
			return v, nil
		}
		return template.VoidValue, fmt.Errorf("wrong number of args")
	}
}

func duration(v interface{}) (time.Duration, error) {
	switch v := v.(type) {
	case time.Duration:
		return v, nil
	case types.Duration:
		return v.Duration(), nil
	case []byte:
		return time.ParseDuration(string(v))
	case string:
		return time.ParseDuration(string(v))
	case int64:
		return time.Duration(int64(v)), nil
	case uint:
		return time.Duration(int64(v)), nil
	case uint64:
		return time.Duration(int64(v)), nil
	case int:
		return time.Duration(int64(v)), nil
	}
	return 0, fmt.Errorf("cannot convert to duration: %v", v)
}

type errExpired string

func (err errExpired) Error() string {
	return fmt.Sprintf("expired key:%v", string(err))
}

// IsExpired returns true if the error is from wait expired / timeout
func IsExpired(err error) bool {
	_, is := err.(errExpired)
	return is
}

type errReadonly string

func (err errReadonly) Error() string {
	return fmt.Sprintf("readonly:%v", string(err))
}

// IsReadonly returns true if the error is from wait expired / timeout
func IsReadonly(err error) bool {
	_, is := err.(errReadonly)
	return is
}

// blocking get from metadata. block the same go routine / thread until timeout or value is available
func doBlockingGet(resolver scope.MetadataResolver, path string, retry, timeout time.Duration) (interface{}, error) {

	call, err := resolver(path)
	if err != nil {
		return nil, err
	}

	if call == nil {
		return nil, nil
	}

	// If the key is nil, the query (path) was for existence of the plugin itself
	if types.NullPath.Equal(call.Key) {
		return true, nil
	}

	var value interface{}
	expiry := time.Now().Add(timeout)

	for i := 0; ; i++ {

		var any *types.Any
		any, err = call.Plugin.Get(call.Key)
		if err == nil && any != nil {
			err = any.Decode(&value)
			if err != nil {
				return any.String(), err // note the type changed to string in error return
			}
			return value, err
		}

		if i > 0 && time.Now().After(expiry) {
			err = errExpired(call.Key.String())
			break
		}

		if retry > 0 {
			<-time.After(retry)
		} else {
			break
		}
	}
	return value, err
}

func doSet(resolver scope.MetadataResolver, path string, newValue interface{}) (interface{}, error) {

	call, err := resolver(path)
	if err != nil {
		return nil, err
	}

	if call == nil {
		return nil, nil
	}

	// If the key is nil, the query (path) was for existence of the plugin itself
	if types.NullPath.Equal(call.Key) {
		return true, nil
	}

	var value interface{}
	any, err := call.Plugin.Get(call.Key)
	if err != nil {
		return nil, err
	}

	if any != nil {
		err = any.Decode(&value)
		if err != nil {
			return any.String(), err // note the type changed to string in error return
		}
	}
	any, err = types.AnyValue(newValue)
	if err != nil {
		return value, err
	}

	updatablePlugin, is := call.Plugin.(metadata.Updatable)
	if !is {
		return value, errReadonly(path)
	}
	_, proposed, cas, err := updatablePlugin.Changes([]metadata.Change{
		{
			Path:  call.Key,
			Value: any,
		},
	})
	err = updatablePlugin.Commit(proposed, cas)
	return template.VoidValue, err
}
