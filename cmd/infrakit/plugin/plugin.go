package plugin

import (
	"fmt"
	"io/ioutil"
	sys_os "os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/launch"
	"github.com/docker/infrakit/pkg/launch/inproc"
	"github.com/docker/infrakit/pkg/launch/os"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"

	// Load the inprocess plugins supported
	_ "github.com/docker/infrakit/pkg/run/group"
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
	doWait := start.Flags().BoolP("wait", "w", false, "True to wait in the foreground; Ctrl-C to exit")

	templateFlags, toJSON, _, processTemplate := base.TemplateProcessor(plugins)
	start.Flags().AddFlagSet(templateFlags)

	start.RunE = func(c *cobra.Command, args []string) error {

		buff, err := processTemplate(*configURL)
		if err != nil {
			return err
		}

		view, err := toJSON([]byte(buff))
		if err != nil {
			return err
		}

		configs := types.AnyBytes(view)

		parsedRules := []launch.Rule{}
		err = configs.Decode(&parsedRules)
		if err != nil {
			return err
		}

		// launch plugin via os process
		osExec, err := os.NewLauncher("os")
		if err != nil {
			return err
		}
		// launch docker run, implemented by the same os executor. We just search for a different key (docker-run)
		dockerExec, err := os.NewLauncher("docker-run")
		if err != nil {
			return err
		}
		// launch inprocess plugins
		inprocExec, err := inproc.NewLauncher("inproc", plugins)
		if err != nil {
			return err
		}

		monitor := launch.NewMonitor([]launch.Exec{
			osExec,
			dockerExec,
			inprocExec,
		}, launch.MergeRules(parsedRules, inproc.Rules()))

		startPlugin, err := monitor.Start()
		if err != nil {
			return err
		}

		// This is the channel to send signal that plugins are stopped out of band so stop waiting.
		noRunningPlugins := make(chan struct{})
		// This is the channel for completion of waiting.
		waitDone := make(chan struct{})
		// This is the channel to stop scanning for running plugins.
		pluginScanDone := make(chan struct{})

		var wait sync.WaitGroup
		if *doWait {
			wait.Add(1)
			go func() {
				wait.Wait() // wait for everyone to complete
				close(waitDone)
			}()
		}

		// Now start all the plugins
		started := []string{}

		// We do a count of the plugins running before we start.
		var before, after = 0, 0

		if m, err := plugins().List(); err != nil {
			log.Warn("Problem listing current plugins, continue", "err", err)
		} else {
			before = len(m)
		}

		for _, pluginToStartSpec := range args {
			fmt.Println("Starting up", pluginToStartSpec)

			wait.Add(1)

			p := strings.SplitN(pluginToStartSpec, "=", 2)
			execName := "inproc" // default is to use inprocess goroutine for running plugins
			if len(p) > 1 {
				execName = p[1]
			}
			pluginToStart := p[0]

			startPlugin <- launch.StartPlugin{
				Plugin: plugin.Name(pluginToStart),
				Exec:   launch.ExecName(execName),
				Started: func(config *types.Any) {
					fmt.Println(pluginToStart, "started.")

					started = append(started, pluginToStart)
					wait.Done()
				},
				Error: func(config *types.Any, err error) {
					fmt.Println("Error starting", pluginToStart, "err=", err)
					wait.Done()
				},
			}
		}

		if m, err := plugins().List(); err == nil {
			after = len(m)
		}

		// Here we scan the plugins.  If we are starting up the plugins, wait a little bit
		// for them to show up.  Then we start scanning to see if the sockets are gone.
		// If the sockets are gone, then we can safely exit.
		if *doWait {
			go func() {
				interval := 5 * time.Second

				now := after
				if now <= before {
					// Here we have fewer plugins running then before. Wait a bit
					time.Sleep(interval)
				}
				checkNow := time.Tick(interval)
				for {
					select {
					case <-pluginScanDone:
						log.Info("--wait mode: stop scanning.")
						return

					case <-checkNow:
						if m, err := plugins().List(); err == nil {
							now = len(m)
						}
						if now == 0 {
							log.Info("--wait mode: scan found no plugins.")
							close(noRunningPlugins)
						}
					}
				}
			}()
		}

		// Here we wait for either wait group to be done or if they are killed out of band.
		select {
		case <-waitDone:
			log.Info("All plugins completed. Exiting.")
		case <-noRunningPlugins:
			log.Info("Plugins aren't running anymore.  Exiting.")
		}

		monitor.Stop()

		close(pluginScanDone)
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
