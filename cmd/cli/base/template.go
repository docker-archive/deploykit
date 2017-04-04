package base

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/docker/infrakit/pkg/discovery"
	metadata_template "github.com/docker/infrakit/pkg/plugin/metadata/template"
	"github.com/docker/infrakit/pkg/template"
	"github.com/ghodss/yaml"
	"github.com/spf13/pflag"
)

// ProcessTemplateFunc is the function that processes the template at url and returns view or error.
type ProcessTemplateFunc func(url string) (rendered string, err error)

// ToJSONFunc converts the input buffer to json format
type ToJSONFunc func(in []byte) (json []byte, err error)

// FromJSONFunc converts json formatted input to output buffer
type FromJSONFunc func(json []byte) (out []byte, err error)

// ReadFromStdinIfElse checks condition and reads from stdin if true; otherwise it executes other.
func ReadFromStdinIfElse(
	condition func() bool,
	otherwise func() (string, error),
	toJSON ToJSONFunc) (rendered string, err error) {

	if condition() {
		buff, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		json, err := toJSON(buff)
		log.Debug("stdin", "buffer", string(json))
		return string(json), err
	}
	rendered, err = otherwise()
	if err != nil {
		return
	}
	var buff []byte
	buff, err = toJSON([]byte(rendered))
	if err != nil {
		return
	}
	return string(buff), nil
}

// TemplateProcessor returns a flagset and a function for processing template input.
func TemplateProcessor(plugins func() discovery.Plugins) (*pflag.FlagSet, ToJSONFunc, FromJSONFunc, ProcessTemplateFunc) {

	fs := pflag.NewFlagSet("template", pflag.ExitOnError)

	globals := fs.StringSliceP("global", "g", []string{}, "key=value pairs of 'global' values in template")
	yamlDoc := fs.BoolP("yaml", "y", false, "True if input is in yaml format; json is the default")
	dump := fs.BoolP("dump", "x", false, "True to dump to output instead of executing")

	return fs,
		// ToJSONFunc
		func(in []byte) (json []byte, err error) {

			defer func() {

				if *dump {
					fmt.Println("Raw:")
					fmt.Println(string(in))
					fmt.Println("Converted")
					fmt.Println(string(json))
					os.Exit(0) // special for debugging
				}
			}()

			if *yamlDoc {
				json, err = yaml.YAMLToJSON(in)
				return
			}
			json = in
			return

		},
		// FromJSONFunc
		func(json []byte) (out []byte, err error) {

			defer func() {

				if *dump {
					fmt.Println("Raw:")
					fmt.Println(string(json))
					fmt.Println("Converted")
					fmt.Println(string(out))
					os.Exit(0) // special for debugging
				}
			}()

			if *yamlDoc {
				out, err = yaml.JSONToYAML(json)
				return
			}
			out = json
			return

		},
		// ProcessTemplateFunc
		func(url string) (view string, err error) {

			if !strings.Contains(url, "://") {
				p := url
				if dir, err := os.Getwd(); err == nil {
					p = path.Join(dir, url)
				}
				url = "file://" + p
			}

			log.Debug("reading template", "url", url)
			engine, err := template.NewTemplate(url, template.Options{})
			if err != nil {
				return
			}

			for _, global := range *globals {
				kv := strings.SplitN(global, "=", 2)
				if len(kv) != 2 {
					log.Warn("bad format kv", "input", global)
					continue
				}
				key := strings.TrimSpace(kv[0])
				val := strings.TrimSpace(kv[1])
				if key != "" && val != "" {
					engine.Global(key, val)
				}
			}

			engine.WithFunctions(func() []template.Function {
				return []template.Function{
					{
						Name: "metadata",
						Description: []string{
							"Metadata function takes a path of the form \"plugin_name/path/to/data\"",
							"and calls GET on the plugin with the path \"path/to/data\".",
							"It's identical to the CLI command infrakit metadata cat ...",
						},
						Func: metadata_template.MetadataFunc(plugins),
					},
					{
						Name: "resource",
						Func: func(s string) string {
							return fmt.Sprintf("{{ resource `%s` }}", s)
						},
					},
				}
			})

			view, err = engine.Render(nil)
			if err != nil {
				return
			}

			log.Debug("rendered", "view", view)
			if *dump {
				fmt.Println("Final:")
				fmt.Println(string(view))
				os.Exit(0)
			}
			return
		}
}
