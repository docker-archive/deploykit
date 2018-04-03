package plugin

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/docker/infrakit/cmd/infrakit/base"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/launch"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/run/manager"
	"github.com/docker/infrakit/pkg/run/scope"
	group_kind "github.com/docker/infrakit/pkg/run/v0/group"
	manager_kind "github.com/docker/infrakit/pkg/run/v0/manager"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/plugin")

func init() {
	base.Register(Command)
}

// Command is the entrypoint
func Command(scope scope.Scope) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List available plugins",
	}
	quiet := ls.Flags().BoolP("quiet", "q", false, "Print rows without column headers")
	ls.RunE = func(c *cobra.Command, args []string) error {
		entries, err := scope.Plugins().List()
		if err != nil {
			return err
		}

		type ep struct {
			name   string
			listen string
			spi    string
		}

		view := map[string]ep{} // table of name, listen, and spi
		keys := []string{}      // slice of names to sort later, keys into view

		// Show the interfaces implemented by each plugin
		for major, entry := range entries {
			hs, err := client.NewHandshaker(entry.Address)
			if err != nil {
				log.Warn("handshaker error", "err", err, "addr", entry.Address)
				continue
			}

			typeMap, err := hs.Hello()
			if err != nil {
				log.Warn("cannot get types for this kind", "err", err, "addr", entry.Address)
				continue
			}

			for spi, names := range typeMap {

				interfaceSpec := spi

				for _, obj := range names {

					minor := obj.Name
					n := major

					if minor != "." {
						n = n + "/" + minor
					}

					ep := ep{
						name:   n,
						listen: entry.Address,
						spi:    interfaceSpec.Encode(),
					}

					key := fmt.Sprintf("%s:%s", ep.name, ep.spi)
					view[key] = ep
					keys = append(keys, key)
				}
			}
		}

		if !*quiet {
			fmt.Printf("%-20s%-30s%-s\n", "INTERFACE", "NAME", "LISTEN")
		}

		sort.Strings(keys)

		for _, k := range keys {

			ep := view[k]
			fmt.Printf("%-20s%-30s%-s\n", ep.spi, ep.name, ep.listen)

		}

		return nil
	}

	start := &cobra.Command{
		Use:   "start",
		Short: "Start named plugins. Args are a list of plugin names",
	}

	configURL := start.Flags().String("config-url", "", "URL for the startup configs")

	services := cli.NewServices(scope)
	start.Flags().AddFlagSet(services.ProcessTemplateFlags)

	start.RunE = func(c *cobra.Command, args []string) error {

		log.Info("config", "url", *configURL)
		pluginManager, err := cli.PluginManager(scope, services, *configURL)
		if err != nil {
			return err
		}

		if len(args) == 0 {

			fmt.Println("Plugins available:")
			fmt.Printf("%-20s\t%s\n", "KIND", "EXEC")
			for _, r := range pluginManager.Rules() {
				execs := []string{}
				for k := range r.Launch {
					execs = append(execs, string(k))
				}
				fmt.Printf("%-20v\t%v\n", r.Key, strings.Join(execs, ","))
			}
			return nil
		}

		defer func() {
			if r := recover(); r != nil {
				log.Error("Error occurred. Recovered but exiting.", "err", r)
				pluginManager.TerminateRunning()
			}
			pluginManager.WaitForAllShutdown()
			log.Info("All plugins shutdown")
			pluginManager.Stop()
		}()

		// Generate a list of StartPlugin instructions for each arg that isn't seen in discovery
		for _, arg := range args {

			p := strings.Split(arg, "=")
			execName := "inproc" // default is to use inprocess goroutine for running plugins
			if len(p) > 1 {
				execName = p[1]
			}

			// the format is kind[:{plugin_name}][={os|inproc}]
			pp := strings.Split(p[0], ":")
			kind := pp[0]
			name := plugin.Name(kind)

			// This is some special case for the legacy setup (pre v0.6)
			switch kind {
			case manager_kind.Kind:
				name = plugin.Name(manager_kind.LookupName)
			case group_kind.Kind:
				name = plugin.Name(group_kind.LookupName)
			}

			// customized by user as override
			if len(pp) > 1 {
				name = plugin.Name(pp[1])
			}

			log.Info("Launching", "kind", kind, "name", name)
			err = pluginManager.Launch(execName, kind, name, nil)
			if err != nil {
				log.Warn("failed to launch", "exec", execName, "kind", kind, "name", name)
			}
		}

		pluginManager.WaitStarting()

		log.Info("Done waiting on plugin starts")

		return nil
	}

	stop := &cobra.Command{
		Use:   "stop",
		Short: "Stop named plugins. Args are a list of plugin names.  This assumes plugins are local processes and not containers managed by another daemon, like Docker or runc.",
	}

	all := stop.Flags().Bool("all", false, "True to stop all running plugins")
	stop.RunE = func(c *cobra.Command, args []string) error {

		pluginManager, err := manager.ManagePlugins([]launch.Rule{}, scope, false, 5*time.Second)
		if err != nil {
			return err
		}

		if *all {
			return pluginManager.TerminateAll()
		}
		return pluginManager.Terminate(args)
	}

	cmd.AddCommand(ls, start, stop)

	return cmd
}
