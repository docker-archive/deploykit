package plugin

import (
	"fmt"
	"io/ioutil"
	sys_os "os"
	"path"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/launch"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/run/manager"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"

	// Load the inprocess plugins supported
	_ "github.com/docker/infrakit/pkg/run/v1/controller/group"
	_ "github.com/docker/infrakit/pkg/run/v1/flavor/swarm"
	_ "github.com/docker/infrakit/pkg/run/v1/flavor/vanilla"
	_ "github.com/docker/infrakit/pkg/run/v1/instance/file"
	_ "github.com/docker/infrakit/pkg/run/v1/manager"
)

var log = logutil.New("module", "cli/plugin")

func init() {
	base.Register(Command)
}

// Command is the entrypoint
func Command(plugins func() discovery.Plugins) *cobra.Command {

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
		entries, err := plugins().List()
		if err != nil {
			return err
		}

		type ep struct {
			name   string
			listen string
			spi    rpc.InterfaceSpec
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

			typeMap, err := hs.Types()
			if err != nil {
				log.Warn("cannot get types for this kind", "err", err, "addr", entry.Address)

				// plugins that haven't been updated to the new types() call

				// try the implements
				if spis, err := hs.Implements(); err == nil {
					for _, spi := range spis {
						ep := ep{
							name:   major,
							listen: entry.Address,
							spi:    rpc.InterfaceSpec(fmt.Sprintf("%s/%s", spi.Name, spi.Version)),
						}

						key := fmt.Sprintf("%s:%s", ep.name, ep.spi)
						view[key] = ep
						keys = append(keys, key)
					}
				} else {
					ep := ep{
						name:   major,
						listen: entry.Address,
					}
					view[ep.name] = ep
					keys = append(keys, ep.name)
				}
				continue
			}

			for spi, names := range typeMap {

				interfaceSpec := spi

				for _, minor := range names {

					n := major

					if minor != "." {
						n = n + "/" + minor
					}

					ep := ep{
						name:   n,
						listen: entry.Address,
						spi:    interfaceSpec,
					}

					key := fmt.Sprintf("%s:%s", ep.name, ep.spi)
					view[key] = ep
					keys = append(keys, key)
				}
			}
		}

		if !*quiet {
			fmt.Printf("%-20s%-50s%-s\n", "INTERFACE", "LISTEN", "NAME")
		}

		sort.Strings(keys)

		for _, k := range keys {

			ep := view[k]
			fmt.Printf("%-20s%-50s%-s\n", ep.spi, ep.listen, ep.name)

		}

		return nil
	}

	start := &cobra.Command{
		Use:   "start",
		Short: "Start named plugins. Args are a list of plugin names",
	}

	configURL := start.Flags().String("config-url", "", "URL for the startup configs")
	mustAll := start.Flags().Bool("all", true, "Panic if any plugin fails to start")

	templateFlags, toJSON, _, processTemplate := base.TemplateProcessor(plugins)
	start.Flags().AddFlagSet(templateFlags)

	start.RunE = func(c *cobra.Command, args []string) error {

		if plugins == nil {
			panic("no plugins()")
		}

		parsedRules := []launch.Rule{}

		if *configURL != "" {
			buff, err := processTemplate(*configURL)
			if err != nil {
				return err
			}

			view, err := toJSON([]byte(buff))
			if err != nil {
				return err
			}

			configs := types.AnyBytes(view)
			err = configs.Decode(&parsedRules)
			if err != nil {
				return err
			}
		}

		pluginManager, err := manager.ManagePlugins(parsedRules, plugins, *mustAll, 5*time.Second)
		if err != nil {
			return err
		}

		// Generate a list of StartPlugin instructions for each arg that isn't seen in discovery
		for _, arg := range args {

			p := strings.Split(arg, "=")
			execName := "inproc" // default is to use inprocess goroutine for running plugins
			if len(p) > 1 {
				execName = p[1]
			}
			pluginToStart := p[0]

			err = pluginManager.Launch(execName, plugin.Name(pluginToStart), nil)
			if err != nil {
				log.Warn("failed to launch", "exec", execName, "plugin", pluginToStart)
			}
		}

		pluginManager.WaitStarting()

		log.Info("Done waiting on plugin starts")

		pluginManager.WaitForAllShutdown()
		log.Info("All plugins shutdown")

		pluginManager.Stop()
		return nil
	}

	stop := &cobra.Command{
		Use:   "stop",
		Short: "Stop named plugins. Args are a list of plugin names.  This assumes plugins are local processes and not containers managed by another daemon, like Docker or runc.",
	}

	all := stop.Flags().Bool("all", false, "True to stop all running plugins")
	stop.RunE = func(c *cobra.Command, args []string) error {

		allPlugins, err := plugins().List()
		if err != nil {
			return err
		}

		targets := args

		if *all {
			names := []string{}
			for n := range allPlugins {
				names = append(names, n)
			}
			targets = names
		}

		for _, n := range targets {

			p, has := allPlugins[n]
			if !has {
				continue
			}

			pidFile := n + ".pid"
			if p.Protocol == "unix" {
				pidFile = p.Address + ".pid"
			} else {
				pidFile = path.Join(local.Dir(), pidFile)
			}

			buff, err := ioutil.ReadFile(pidFile)
			if err != nil {
				log.Warn("Cannot read PID file", "name", n, "pid", pidFile)
				continue
			}

			pid, err := strconv.Atoi(string(buff))
			if err != nil {
				log.Warn("Cannot determine PID", "name", n, "pid", pidFile)
				continue
			}

			process, err := sys_os.FindProcess(pid)
			if err != nil {
				log.Warn("Error finding process of plugin", "name", n)
				continue
			}

			log.Info("Stopping", "name", n, "pid", pid)
			if err := process.Signal(syscall.SIGTERM); err == nil {
				process.Wait()
				log.Info("Process exited", "name", n)
			}

		}

		return nil
	}

	cmd.AddCommand(ls, start, stop)

	return cmd
}

// counts the number of matches by name
func countMatches(list []string, found map[string]*plugin.Endpoint) int {
	c := 0
	for _, l := range list {
		if _, has := found[l]; has {
			log.Debug("Scan found", "lookup", l)
			c++
		}
	}
	return c
}
