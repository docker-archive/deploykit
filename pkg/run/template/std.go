package template

import (
	"fmt"
	"strings"

	"github.com/docker/infrakit/pkg/discovery"
	metadata_template "github.com/docker/infrakit/pkg/plugin/metadata/template"
	"github.com/docker/infrakit/pkg/template"
)

// StdFunctions adds a set of standard functions for access in templates
func StdFunctions(engine *template.Template, plugins func() discovery.Plugins) *template.Template {
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
			// This is an override of the existing Var function
			{
				Name: "var",
				Func: func(name string, optional ...interface{}) (interface{}, error) {

					// returns nil if it's a read and unresolved
					// or if it's a write, returns a void value that is not nil (an empty string)
					v := engine.Var(name, optional...)
					switch {
					case len(optional) > 0: // writing
						return v, nil
					case v != nil && !engine.Options().MultiPass: // reading, resolved
						return v, nil
					}
					// Check to see if the value is resolved...  in the case of multipass, we can get `{{ var foo }}` back
					if engine.Options().MultiPass {
						delim := `{{`
						if engine.Options().DelimLeft != "" {
							delim = engine.Options().DelimLeft
						}
						if strings.Index(fmt.Sprintf("%v", v), delim) < 0 {
							return v, nil
						}
						// If it's multipass and we have a template expression returned, then the value is not resolved.
						// continue to look up as metadata
					}

					// If not resolved, try to interpret the path as a path for metadata...
					return metadata_template.MetadataFunc(plugins)(name, optional...)
				},
			},
		}
	})
	return engine
}
