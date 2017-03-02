package main

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	metadata_template "github.com/docker/infrakit/pkg/plugin/metadata/template"
	"github.com/docker/infrakit/pkg/template"
	"github.com/spf13/cobra"
)

func templateCommand(plugins func() discovery.Plugins) *cobra.Command {

	globals := []string{}
	templateURL := ""
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Render an infrakit template",
		RunE: func(cmd *cobra.Command, args []string) error {

			log.Infof("Using %v for reading template\n", templateURL)
			engine, err := template.NewTemplate(templateURL, template.Options{
				SocketDir: discovery.Dir(),
			})
			if err != nil {
				return err
			}

			// Add functions
			for _, global := range globals {
				kv := strings.Split(global, "=")
				if len(kv) != 2 {
					continue
				}
				key := strings.Trim(kv[0], " \t\n")
				val := strings.Trim(kv[1], " \t\n")
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
				}
			})
			view, err := engine.Render(nil)
			if err != nil {
				return err
			}

			fmt.Print(view)
			return nil
		},
	}
	cmd.Flags().StringVar(&templateURL, "url", "", "URL for the template")
	cmd.Flags().StringSliceVar(&globals, "global", []string{}, "key=value pairs of 'global' values")

	return cmd
}
