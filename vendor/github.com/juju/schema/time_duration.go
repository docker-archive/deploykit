// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package schema

import (
	"reflect"
	"time"
)

// TimeDuration returns a Checker that accepts a string value, and returns
// the parsed time.Duration value. Emtpy strings are considered empty time.Duration
func TimeDuration() Checker {
	return timeDurationC{}
}

type timeDurationC struct{}

// Coerce implements Checker Coerce method.
func (c timeDurationC) Coerce(v interface{}, path []string) (interface{}, error) {
	if v == nil {
		return nil, error_{"string or time.Duration", v, path}
	}

	var empty time.Duration
	switch reflect.TypeOf(v).Kind() {
	case reflect.TypeOf(empty).Kind():
		return v, nil
	case reflect.String:
		vstr := reflect.ValueOf(v).String()
		if vstr == "" {
			return empty, nil
		}
		v, err := time.ParseDuration(vstr)
		if err != nil {
			return nil, err
		}
		return v, nil
	default:
		return nil, error_{"string or time.Duration", v, path}
	}
}
