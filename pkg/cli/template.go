package cli

import (
	"fmt"

	"github.com/docker/infrakit/pkg/discovery"
	metadata_template "github.com/docker/infrakit/pkg/plugin/metadata/template"
	"github.com/docker/infrakit/pkg/template"
)

// ConfigureTemplate is a utility that helps setup template engines in a standardized way across all uses.
func ConfigureTemplate(engine *template.Template, plugins func() discovery.Plugins) *template.Template {
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
	return engine
}
