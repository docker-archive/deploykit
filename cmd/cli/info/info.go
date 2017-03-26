package info

import (
	"encoding/json"
	"fmt"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/template"
	"github.com/spf13/cobra"
)

// Command creates a cobra Command that prints build version information.
func Command(plugins func() discovery.Plugins) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "print plugin info",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			return cli.EnsurePersistentPreRunE(c)
		},
	}
	name := cmd.PersistentFlags().String("name", "", "Name of plugin")
	raw := cmd.PersistentFlags().Bool("raw", false, "True to show raw data")

	api := &cobra.Command{
		Use:   "api",
		Short: "Show api / RPC interface supported by the plugin of the given name",
	}

	templateFuncs := &cobra.Command{
		Use:   "template",
		Short: "Show template functions supported by the plugin, if the plugin uses template for configuration.",
	}
	cmd.AddCommand(api, templateFuncs)

	api.RunE = func(cmd *cobra.Command, args []string) error {
		endpoint, err := plugins().Find(plugin.Name(*name))
		if err != nil {
			return err
		}

		infoClient := client.NewPluginInfoClient(endpoint.Address)
		info, err := infoClient.GetInfo()
		if err != nil {
			return err
		}

		if *raw {
			buff, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(buff))
			return nil
		}
		// render a view using template
		renderer, err := template.NewTemplate("str://"+apiViewTemplate, template.Options{})
		if err != nil {
			return err
		}

		view, err := renderer.Def("plugin", *name, "Plugin name").Render(info)
		if err != nil {
			return err
		}

		fmt.Print(view)
		return nil
	}

	templateFuncs.RunE = func(cmd *cobra.Command, args []string) error {
		endpoint, err := plugins().Find(plugin.Name(*name))
		if err != nil {
			return err
		}

		infoClient := client.NewPluginInfoClient(endpoint.Address)
		info, err := infoClient.GetFunctions()
		if err != nil {
			return err
		}

		if *raw {
			buff, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(buff))
			return nil
		}
		// render a view using template
		renderer, err := template.NewTemplate("str://"+funcsViewTemplate, template.Options{})
		if err != nil {
			return err
		}

		view, err := renderer.Def("plugin", *name, "Plugin name").Render(info)
		if err != nil {
			return err
		}

		fmt.Print(view)
		return nil
	}

	return cmd
}

const (
	apiViewTemplate = `
Plugin:     {{ref "plugin"}}
Implements: {{range $spi := .Implements}}{{$spi.Name}}/{{$spi.Version}} {{end}}
Interfaces: {{range $iface := .Interfaces}}
  SPI:      {{$iface.Name}}/{{$iface.Version}}
  RPC:      {{range $method := $iface.Methods}}
    Method: {{$method.Request | q "method" }}
    Request:
    {{$method.Request | to_json_format "    " "  "}}

    Response:
    {{$method.Response | to_json_format "    " "  "}}

    -------------------------
  {{end}}
{{end}}
`

	funcsViewTemplate = `
{{range $category, $functions := .}}
{{ref "plugin"}}/{{$category}} _________________________________________________________________________________________
  {{range $f := $functions}}
    Name:        {{$f.Name}}
    Description: {{join "\n                 " $f.Description}}
    Function:    {{$f.Function}}
    Usage:       {{$f.Usage}}

    -------------------------
  {{end}}

{{end}}
`
)
