package v0

import (
	"encoding/json"
	"fmt"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/template"
	"github.com/spf13/cobra"

	// v0 loads these packages
	_ "github.com/docker/infrakit/pkg/cli/v0/controller"
	_ "github.com/docker/infrakit/pkg/cli/v0/event"
	_ "github.com/docker/infrakit/pkg/cli/v0/flavor"
	_ "github.com/docker/infrakit/pkg/cli/v0/group"
	_ "github.com/docker/infrakit/pkg/cli/v0/instance"
	_ "github.com/docker/infrakit/pkg/cli/v0/loadbalancer"
	_ "github.com/docker/infrakit/pkg/cli/v0/manager"
	_ "github.com/docker/infrakit/pkg/cli/v0/metadata"
	_ "github.com/docker/infrakit/pkg/cli/v0/resource"
)

func init() {
	cli.Register(controller.InterfaceSpec,
		[]cli.CmdBuilder{
			Info,
		})

	cli.Register(instance.InterfaceSpec,
		[]cli.CmdBuilder{
			Info,
		})
	cli.Register(flavor.InterfaceSpec,
		[]cli.CmdBuilder{
			Info,
		})
	cli.Register(group.InterfaceSpec,
		[]cli.CmdBuilder{
			Info,
		})
	cli.Register(resource.InterfaceSpec,
		[]cli.CmdBuilder{
			Info,
		})
	cli.Register(metadata.InterfaceSpec,
		[]cli.CmdBuilder{
			Info,
		})
}

// Info returns the info command which works for multiple interfaces (e.g. instance, flavor, etc.)
func Info(name string, services *cli.Services) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "print plugin info",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			return cli.EnsurePersistentPreRunE(c)
		},
	}

	raw := cmd.Flags().Bool("raw", false, "True to show raw data")

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
		endpoint, err := services.Scope.Plugins().Find(plugin.Name(name))
		if err != nil {
			return err
		}

		infoClient, err := client.NewPluginInfoClient(endpoint.Address)
		if err != nil {
			return err
		}

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

		view, err := renderer.Global("plugin", name).Render(info)
		if err != nil {
			return err
		}

		fmt.Print(view)
		return nil
	}

	templateFuncs.RunE = func(cmd *cobra.Command, args []string) error {
		endpoint, err := services.Scope.Plugins().Find(plugin.Name(name))
		if err != nil {
			return err
		}

		infoClient, err := client.NewPluginInfoClient(endpoint.Address)
		if err != nil {
			return err
		}
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

		view, err := renderer.Global("plugin", name).Render(info)
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
Plugin:     {{var "plugin"}}
Implements: {{range $spi := .Implements}}{{$spi.Name}}/{{$spi.Version}} {{end}}
Interfaces: {{range $iface := .Interfaces}}
  SPI:      {{$iface.Name}}/{{$iface.Version}}
  RPC:      {{range $method := $iface.Methods}}
    Method: {{$method.Request | q "method" }}
    Request:
    {{$method.Request | jsonEncode | yamlEncode }}

    Response:
    {{$method.Response | jsonEncode | yamlEncode }}

    -------------------------
  {{end}}
{{end}}
`

	funcsViewTemplate = `
{{range $category, $functions := .}}
{{var "plugin"}}/{{$category}} _________________________________________________________________________________________
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
