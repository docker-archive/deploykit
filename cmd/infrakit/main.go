package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/cli/playbook"
	"github.com/docker/infrakit/pkg/discovery"
	discovery_local "github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/discovery/remote"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/spf13/cobra"

	// CLI commands
	_ "github.com/docker/infrakit/cmd/infrakit/manager"
	_ "github.com/docker/infrakit/cmd/infrakit/playbook"
	_ "github.com/docker/infrakit/cmd/infrakit/plugin"
	_ "github.com/docker/infrakit/cmd/infrakit/remote"
	_ "github.com/docker/infrakit/cmd/infrakit/template"
	_ "github.com/docker/infrakit/cmd/infrakit/up"
	_ "github.com/docker/infrakit/cmd/infrakit/util"
	_ "github.com/docker/infrakit/cmd/infrakit/x"
	_ "github.com/docker/infrakit/pkg/cli/v0"

	// CLI backends
	_ "github.com/docker/infrakit/pkg/cli/backend/http"
	_ "github.com/docker/infrakit/pkg/cli/backend/instance"
	_ "github.com/docker/infrakit/pkg/cli/backend/manager"
	_ "github.com/docker/infrakit/pkg/cli/backend/print"
	_ "github.com/docker/infrakit/pkg/cli/backend/sh"

	// Supported "kinds"
	_ "github.com/docker/infrakit/pkg/run/v0/aws"
	_ "github.com/docker/infrakit/pkg/run/v0/combo"
	_ "github.com/docker/infrakit/pkg/run/v0/digitalocean"
	_ "github.com/docker/infrakit/pkg/run/v0/docker"
	_ "github.com/docker/infrakit/pkg/run/v0/enrollment"
	_ "github.com/docker/infrakit/pkg/run/v0/file"
	_ "github.com/docker/infrakit/pkg/run/v0/google"
	_ "github.com/docker/infrakit/pkg/run/v0/group"
	_ "github.com/docker/infrakit/pkg/run/v0/hyperkit"
	_ "github.com/docker/infrakit/pkg/run/v0/ibmcloud"
	_ "github.com/docker/infrakit/pkg/run/v0/image"
	_ "github.com/docker/infrakit/pkg/run/v0/ingress"
	_ "github.com/docker/infrakit/pkg/run/v0/kubernetes"
	_ "github.com/docker/infrakit/pkg/run/v0/maas"
	_ "github.com/docker/infrakit/pkg/run/v0/manager"
	_ "github.com/docker/infrakit/pkg/run/v0/oneview"
	_ "github.com/docker/infrakit/pkg/run/v0/oracle"
	_ "github.com/docker/infrakit/pkg/run/v0/packet"
	_ "github.com/docker/infrakit/pkg/run/v0/rackhd"
	_ "github.com/docker/infrakit/pkg/run/v0/selector"
	_ "github.com/docker/infrakit/pkg/run/v0/simulator"
	_ "github.com/docker/infrakit/pkg/run/v0/swarm"
	_ "github.com/docker/infrakit/pkg/run/v0/tailer"
	_ "github.com/docker/infrakit/pkg/run/v0/terraform"
	_ "github.com/docker/infrakit/pkg/run/v0/time"
	_ "github.com/docker/infrakit/pkg/run/v0/vagrant"
	_ "github.com/docker/infrakit/pkg/run/v0/vanilla"
	_ "github.com/docker/infrakit/pkg/run/v0/vars"
	_ "github.com/docker/infrakit/pkg/run/v0/vsphere"
)

func init() {
	logutil.Configure(&logutil.ProdDefaults)
}

type emptyPlugins struct{}

func (e emptyPlugins) Find(name plugin.Name) (*plugin.Endpoint, error) {
	return nil, errEmpty
}

func (e emptyPlugins) List() (map[string]*plugin.Endpoint, error) {
	return nil, errEmpty
}

var (
	empty    = emptyPlugins{}
	errEmpty = errors.New("no plugins")

	log = logutil.New("module", "main")
)

// A generic client for infrakit
func main() {

	if err := discovery_local.Setup(); err != nil {
		panic(err)
	}
	if err := template.Setup(); err != nil {
		panic(err)
	}
	// Log setup
	logOptions := &logutil.ProdDefaults

	program := path.Base(os.Args[0])
	cmd := &cobra.Command{
		Use:   program,
		Short: program + " command line interface",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			logutil.Configure(logOptions)
			return nil
		},
	}
	cmd.PersistentFlags().AddFlagSet(cli.Flags(logOptions))
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	// Don't print usage text for any error returned from a RunE function.
	// Only print it when explicitly requested.
	cmd.SilenceUsage = true

	// Don't automatically print errors returned from a RunE function.
	// They are returned from cmd.Execute() below and we print it ourselves.
	cmd.SilenceErrors = true
	f := func() discovery.Plugins {

		ulist, err := cli.Remotes()
		if err != nil {
			log.Debug("Cannot lookup plugins", "err", err)
			return empty
		}

		if len(ulist) == 0 {
			d, err := discovery_local.NewPluginDiscovery()
			if err != nil {
				log.Debug("Failed to initialize plugin discovery", "err", err)
				return empty
			}
			return d
		}

		d, err := remote.NewPluginDiscovery(ulist)
		if err != nil {
			log.Debug("Failed to initialize remote plugin discovery", "err", err)
			return empty
		}
		return d
	}

	scope := scope.DefaultScope(f)

	cmd.AddCommand(cli.VersionCommand())

	// playbooks
	pb, err := playbook.Load()
	if err != nil {
		log.Warn("Cannot load playbook file")
	}

	if !pb.Empty() {
		cmd.AddCommand(useCommand(scope, pb))
	}

	base.VisitModules(scope, func(c *cobra.Command) {
		cmd.AddCommand(c)
	})

	// Set environment variable to disable this feature.
	if os.Getenv("INFRAKIT_DYNAMIC_CLI") != "false" {
		// Load dynamic plugin commands based on discovery
		pluginCommands, err := cli.LoadAll(cli.NewServices(scope))
		if err != nil && err != errEmpty {
			log.Debug("error loading", "cmd", cmd.Use, "err", err)
			fmt.Println(err.Error())
			os.Exit(1)
		}
		for _, c := range pluginCommands {
			cmd.AddCommand(c)
		}
	}

	// Help template includes the usage string, which is configure below
	cmd.SetHelpTemplate(helpTemplate)
	cmd.SetUsageTemplate(usageTemplate)

	err = cmd.Execute()
	if err != nil {
		log.Crit("error executing", "cmd", cmd.Use, "err", err)
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// write the file for bash completion if environment variable is set
	bashCompletionScript := os.Getenv("INFRAKIT_BASH_COMPLETION")
	if bashCompletionScript != "" {
		cmd.GenBashCompletionFile(bashCompletionScript)
	}
}

func useCommand(scope scope.Scope, pb *playbook.Playbooks) *cobra.Command {

	// Log setup
	logOptions := &logutil.ProdDefaults
	logFlags := cli.Flags(logOptions)

	use := "use"
	description := "Use a playbook"

	cmd := &cobra.Command{
		Use:   use,
		Short: description,

		RunE: func(c *cobra.Command, args []string) error {

			main := &cobra.Command{
				Use: path.Base(os.Args[0]),
				PersistentPreRunE: func(c *cobra.Command, args []string) error {
					logutil.Configure(logOptions)
					return nil
				},
			}
			main.PersistentFlags().AddFlagSet(logFlags)

			useCmd := &cobra.Command{
				Use:   use,
				Short: description,
			}
			main.AddCommand(useCmd)

			// Commands from playbooks
			playbookCommands := []*cobra.Command{}
			if playbooks, err := playbook.NewModules(scope, pb.Modules(), os.Stdin, template.Options{}); err != nil {
				log.Warn("error loading playbooks", "err", err)
			} else {
				if more, err := playbooks.List(); err != nil {
					log.Warn("cannot list playbooks", "err", err)
				} else {
					playbookCommands = append(playbookCommands, more...)
				}
			}

			for _, cc := range playbookCommands {
				useCmd.AddCommand(cc)
			}

			main.SetArgs(os.Args[1:])
			return main.Execute()
		},
	}

	return cmd
}

const (
	helpTemplate = `

{{with or .Long .Short }}{{. | trim}}{{end}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}
`

	usageTemplate = `
Usage:{{if .Runnable}}
  {{if .HasAvailableFlags}}{{appendIfNotPresent .UseLine "[flags]"}}{{else}}{{.UseLine}}{{end}}{{end}}{{if .HasAvailableSubCommands}}
  {{ .CommandPath}} [command]{{end}}{{if gt .Aliases 0}}

Aliases:
  {{.NameAndAliases}}
{{end}}{{if .HasExample}}

Examples:
{{ .Example }}{{end}}{{ if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{ if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimRightSpace}}{{end}}{{ if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimRightSpace}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsHelpCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{ if .HasAvailableSubCommands }}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
)
