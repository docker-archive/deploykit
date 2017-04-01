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

// ReadFromStdinIfElse checks condition and reads from stdin if true; otherwise it executes other.
func ReadFromStdinIfElse(condition func() bool, otherwise func() (string, error)) (rendered string, err error) {
	if condition() {
		buff, err := ioutil.ReadAll(os.Stdin)
		return string(buff), err
	}
	return otherwise()
}

// TemplateProcessor returns a flagset and a function for processing template input.
func TemplateProcessor(plugins func() discovery.Plugins) (*pflag.FlagSet, ProcessTemplateFunc) {

	fs := pflag.NewFlagSet("template", pflag.ExitOnError)

	globals := fs.StringSliceP("global", "g", []string{}, "key=value pairs of 'global' values in template")
	yamlDoc := fs.BoolP("yaml", "y", false, "True if input is in yaml format; json is the default")

	return fs, func(url string) (view string, err error) {

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

		if *yamlDoc {

			log.Debug("converting yaml to json")

			// Convert this to json if it's not in JSON -- the default is for the template to be in YAML
			if converted, e := yaml.YAMLToJSON([]byte(view)); err == nil {
				view = string(converted)
			} else {
				err = e
				return
			}
		}
		log.Debug("rendered", "view", view)
		return
	}
}
