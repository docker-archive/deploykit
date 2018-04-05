package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"

	_ "net/http/pprof"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/cli/playbook"
	"github.com/docker/infrakit/pkg/discovery"
	discovery_local "github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/discovery/remote"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/run/local"
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

	// Callable backends via playbook or via lib
	_ "github.com/docker/infrakit/pkg/callable/backend/http"
	_ "github.com/docker/infrakit/pkg/callable/backend/instance"
	_ "github.com/docker/infrakit/pkg/callable/backend/print"
	_ "github.com/docker/infrakit/pkg/callable/backend/sh"
	_ "github.com/docker/infrakit/pkg/callable/backend/ssh"
	_ "github.com/docker/infrakit/pkg/callable/backend/stack"
	_ "github.com/docker/infrakit/pkg/callable/backend/vmwscript"
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
	logFlags := cli.Flags(logOptions)

	program := path.Base(os.Args[0])
	cmd := &cobra.Command{
		Use:   program,
		Short: program + " command line interface",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			logutil.Configure(logOptions)

			if logOptions.Level == 5 {
				// Debug level. Start pprof
				go func() {
					log.Info("Starting pprof at localhost:6060")
					http.ListenAndServe("localhost:6060", nil)
				}()
			}

			return nil
		},
	}
	cmd.PersistentFlags().AddFlagSet(logFlags)

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
		useCommandName := "use"
		useCommandDescription := "Use a playbook"
		cmd.AddCommand(useCommand(scope, useCommandName, useCommandDescription, pb))
		if len(os.Args) > 2 && os.Args[1] == useCommandName {
			cmd.SetArgs(os.Args[1:2])
		}

	}

	base.VisitModules(scope, func(c *cobra.Command) {
		cmd.AddCommand(c)
	})

	// Help template includes the usage string, which is configure below
	cmd.SetHelpTemplate(helpTemplate)
	cmd.SetUsageTemplate(usageTemplate)

	// The 'stack' subcommand has its Use (verb) that is set to the
	// value of the INFRAKIT_HOST env variable.  This allows us to
	// discover the remote services and generate the dynamic commands only
	// when the user has typed
	stackCommandName := local.InfrakitHost()
	stackCommand := stackCommand(scope, stackCommandName)

	cmd.AddCommand(stackCommand)

	if len(os.Args) > 2 && os.Args[1] == stackCommandName {
		cmd.SetArgs(os.Args[1:2])
	}
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

func useCommand(scope scope.Scope, use, description string, pb *playbook.Playbooks) *cobra.Command {

	// Log setup
	logOptions := &logutil.ProdDefaults
	logFlags := cli.Flags(logOptions)

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
			if playbooks, err := playbook.NewModules(scope, pb.Modules(),
				os.Stdin,
				playbook.Options{
					// This value here is set by the environment variables
					// because its evaluation is needed prior to flags generation.
					ShowAllWarnings: local.Getenv("INFRAKIT_CALLABLE_WARNINGS", "false") == "true",
				}); err != nil {
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

func stackCommand(scope scope.Scope, stackCommandName string) *cobra.Command {

	// Log setup
	logOptions := &logutil.ProdDefaults
	logFlags := cli.Flags(logOptions)

	description := fmt.Sprintf("Access %v", stackCommandName)
	cmd := &cobra.Command{
		Use:   stackCommandName,
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

			stack := &cobra.Command{
				Use:   stackCommandName,
				Short: description,
			}
			main.AddCommand(stack)
			// Load dynamic plugin commands based on discovery
			pluginCommands, err := cli.LoadAll(cli.NewServices(scope))
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
			for _, cc := range pluginCommands {
				stack.AddCommand(cc)
			}

			main.SetArgs(os.Args[1:])

			pluginName := ""
			main.SetHelpFunc(func(cc *cobra.Command, args []string) {
				cc.Usage()
				if len(args) > 1 {
					pluginName = args[1]
				}
			})

			err = main.Execute()

			// We want to detect the case when the user typed a plugin that actually doesn't
			// exist and return a non-zero error code to the shell.  This means we have to
			// actually try to lookup the plugin and the object therein.
			if pluginName != "" {
				pn := plugin.Name(pluginName)
				ep, err := scope.Plugins().Find(pn)
				if err != nil {
					os.Exit(1)
				}
				// if endpoint exists, then check if the subtype/object exists
				hk, err := rpc.NewHandshaker(ep.Address)
				if err != nil {
					os.Exit(1)
				}
				hello, err := hk.Hello()
				if err != nil {
					os.Exit(1)
				}

				foundObject := false
				for _, objs := range hello {
					for _, obj := range objs {
						if pn.Type() == obj.Name {
							foundObject = true
							break
						}
					}
				}

				if !foundObject {
					// http://tldp.org/LDP/abs/html/exitcodes.html
					os.Exit(127)
				}
			}
			return err
		},
	}

	return cmd
}
