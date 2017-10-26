package template

import (
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

					if len(optional) > 0 {
						return engine.Var(name, optional...), nil
					}

					v := engine.Ref(name)
					if v == nil {
						// If not resolved, try to interpret the path as a path for metadata...
						m, err := metadata_template.MetadataFunc(plugins)(name, optional...)
						if err != nil {
							return nil, err
						}
						v = m
					}

					if v == nil && engine.Options().MultiPass {
						return engine.DeferVar(name), nil
					}

					return v, nil
				},
			},
		}
	})
	return engine
}
