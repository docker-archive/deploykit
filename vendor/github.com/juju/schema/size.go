// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package schema

import (
	"github.com/juju/utils"
	"reflect"
)

// Size returns a Checker that accepts a string value, and returns
// the parsed string as a size in mebibytes see: https://godoc.org/github.com/juju/utils#ParseSize
func Size() Checker {
	return sizeC{}
}

type sizeC struct{}

// Coerce implements Checker Coerce method.
func (c sizeC) Coerce(v interface{}, path []string) (interface{}, error) {
	if v == nil {
		return nil, error_{"string", v, path}
	}

	typeOf := reflect.TypeOf(v).Kind()
	if typeOf != reflect.String {
		return nil, error_{"string", v, path}
	}

	value := reflect.ValueOf(v).String()
	if value == "" {
		return nil, error_{"empty string", v, path}
	}

	v, err := utils.ParseSize(value)

	if err != nil {
		return nil, err
	}

	return v, nil
}
