package cli

import (
	"fmt"
	"io"

	"github.com/docker/infrakit/pkg/types"
	"github.com/ghodss/yaml"
	"github.com/spf13/pflag"
)

// OutputFunc is a function that writes some data to the output writer
type OutputFunc func(w io.Writer, v interface{}, defaultView func(io.Writer, interface{}) error) (err error)

// Output returns the flagset and the func for printing output
func Output() (*pflag.FlagSet, OutputFunc) {

	fs := pflag.NewFlagSet("output", pflag.ExitOnError)
	raw := fs.BoolP("raw", "r", false, "True to dump raw output")

	yamlDoc := fs.BoolP("yaml", "y", false, "True to output yaml; json is the default")
	return fs, func(w io.Writer, v interface{}, defaultView func(io.Writer, interface{}) error) (err error) {

		if !*raw && defaultView != nil {
			return defaultView(w, v)
		}

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
				return err
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
		return nil
	}
}
