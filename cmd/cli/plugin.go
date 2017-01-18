package main

import (
	"fmt"
	"io/ioutil"
	sys_os "os"
	"strconv"
	"sync"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch"
	"github.com/docker/infrakit/pkg/launch/os"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

func pluginCommand(plugins func() discovery.Plugins) *cobra.Command {

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

		if !*quiet {
			fmt.Printf("%-20s\t%-s\n", "NAME", "LISTEN")
		}
		for k, v := range entries {
			fmt.Printf("%-20s\t%-s\n", k, v.Address)
		}

		return nil
	}

	start := &cobra.Command{
		Use:   "start",
		Short: "Start named plugins. Args are a list of plugin names",
	}

	configURL := start.Flags().String("config-url", "", "URL for the startup configs")
	osExec := start.Flags().Bool("os", false, "True to use os plugin binaries")
	doWait := start.Flags().BoolP("wait", "w", false, "True to wait in the foreground; Ctrl-C to exit")

	start.RunE = func(c *cobra.Command, args []string) error {

		configTemplate, err := template.NewTemplate(*configURL, template.Options{
			SocketDir: discovery.Dir(),
		})
		if err != nil {
			return err
		}

		view, err := configTemplate.Render(nil)
		if err != nil {
			return err
		}

		configs := types.AnyString(view)

		parsedRules := []launch.Rule{}
		err = configs.Decode(&parsedRules)
		if err != nil {
			return err
		}

		monitors := []*launch.Monitor{}

		if *osExec {
			exec, err := os.NewLauncher()
			if err != nil {
				return err
			}
			monitors = append(monitors, launch.NewMonitor(exec, parsedRules))
		}

		input := []chan<- launch.StartPlugin{}
		for _, m := range monitors {
			ch, err := m.Start()
			if err != nil {
				return err
			}
			input = append(input, ch)
		}

		var wait sync.WaitGroup

		if *doWait {
			wait.Add(1)
		}

		// now start all the plugins
		for _, pluginToStart := range args {
			fmt.Println("Starting up", pluginToStart)

			wait.Add(1)

			for _, ch := range input {

				name := pluginToStart
				ch <- launch.StartPlugin{
					Plugin: plugin.Name(name),
					Started: func(config *types.Any) {
						fmt.Println(name, "started.")
						wait.Done()
					},
					Error: func(config *types.Any, err error) {
						fmt.Println("Error starting", name, "err=", err)
						wait.Done()
					},
				}
			}
		}

		wait.Wait() // wait for everyone to complete

		for _, monitor := range monitors {
			monitor.Stop()
		}
		return nil
	}

	stop := &cobra.Command{
		Use:   "stop",
		Short: "Stop named plugins. Args are a list of plugin names.  This assumes plugins are local processes and not containers managed by another daemon, like Docker or runc.",
	}

	stop.RunE = func(c *cobra.Command, args []string) error {

		allPlugins, err := plugins().List()
		if err != nil {
			return err
		}

		for _, n := range args {

			p, has := allPlugins[n]
			if !has {
				log.Warningf("Plugin %s not running", n)
				continue
			}

			if p.Protocol != "unix" {
				log.Warningf("Plugin is not a local process", n)
				continue
			}

			// TODO(chungers) -- here we
			pidFile := p.Address + ".pid"

			buff, err := ioutil.ReadFile(pidFile)
			if err != nil {
				log.Warningf("Cannot read PID file for %s: %s", n, pidFile)
				continue
			}

			pid, err := strconv.Atoi(string(buff))
			if err != nil {
				log.Warningf("Cannot determine PID for %s from file: %s", n, pidFile)
				continue
			}

			process, err := sys_os.FindProcess(pid)
			if err != nil {
				log.Warningf("Error finding process of plugin %s", n)
				continue
			}

			log.Infoln("Stopping", n, "at PID=", pid)
			if err := process.Signal(syscall.SIGTERM); err == nil {
				_, err := process.Wait()
				log.Infoln("Process for", n, "exited")
				if err != nil {
					log.Warningln("error=", err)
				}
			}

		}

		return nil
	}

	cmd.AddCommand(ls, start, stop)

	return cmd
}
