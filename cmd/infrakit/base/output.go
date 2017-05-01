package base

import (
	"fmt"
	"io"

	"github.com/docker/infrakit/pkg/types"
	"github.com/ghodss/yaml"
	"github.com/spf13/pflag"
)

// RawOutputFunc is a function that writes some data to the output writer
type RawOutputFunc func(w io.Writer, v interface{}) (rendered bool, err error)

// RawOutput returns the flagset and the func for printing output
func RawOutput() (*pflag.FlagSet, RawOutputFunc) {

	fs := pflag.NewFlagSet("output", pflag.ExitOnError)

	raw := fs.BoolP("raw", "r", false, "True to dump to output instead of executing")
	yamlDoc := fs.BoolP("yaml", "y", false, "True if input is in yaml format; json is the default")

	return fs, func(w io.Writer, v interface{}) (rendered bool, err error) {
		if !*raw {
			return false, nil
		}

		any, err := types.AnyValue(v)
		if err != nil {
			return false, err
		}

		buff := any.Bytes()
		if *yamlDoc {
			buff, err = yaml.JSONToYAML(buff)
		}

		fmt.Fprintln(w, string(buff))
		return true, nil
	}
}

// OutputFunc is a function that writes some data to the output writer
type OutputFunc func(w io.Writer, v interface{}) (rendered bool, err error)

// Output returns the flagset and the func for printing output
func Output() (*pflag.FlagSet, OutputFunc) {

	fs := pflag.NewFlagSet("output", pflag.ExitOnError)

	yamlDoc := fs.BoolP("yaml", "y", false, "True if input is in yaml format; json is the default")
	return fs, func(w io.Writer, v interface{}) (rendered bool, err error) {

		var out string

		switch v := v.(type) {
		case string:
			if *yamlDoc {
				if y, err := yaml.JSONToYAML([]byte(v)); err == nil {
					out = string(y)
				}
			} else {
				out = v
			}
		case []byte:
			if *yamlDoc {
				if y, err := yaml.JSONToYAML(v); err == nil {
					out = string(y)
				}
			} else {
				out = string(v)
			}
		default:
			any, err := types.AnyValue(v)
			if err != nil {
				return false, err
			}

			buff := any.Bytes()
			if *yamlDoc {
				if y, err := yaml.JSONToYAML(buff); err == nil {
					out = string(y)
				}
			} else {
				out = any.String()
			}
		}

		fmt.Fprintln(w, out)
		return true, nil
	}
}
