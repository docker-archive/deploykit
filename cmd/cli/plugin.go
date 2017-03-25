package main

import (
	"fmt"
	"io/ioutil"
	sys_os "os"
	"strconv"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
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
	executor := start.Flags().String("exec", "os", "Executor to use for starting up plugins: [os | docker-run]")
	doWait := start.Flags().BoolP("wait", "w", false, "True to wait in the foreground; Ctrl-C to exit")

	start.RunE = func(c *cobra.Command, args []string) error {

		configTemplate, err := template.NewTemplate(*configURL, template.Options{
			SocketDir: local.Dir(),
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

		switch *executor {
		case "os":
			exec, err := os.NewLauncher("os")
			if err != nil {
				return err
			}
			monitors = append(monitors, launch.NewMonitor(exec, parsedRules))
		case "docker-run":
			// docker-run is also implemented by the same os executor. We just search for a different key (docker-run)
			exec, err := os.NewLauncher("docker-run")
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
			log.Warningln("Problem listing current plugins:", err, "continue.")
		} else {
			before = len(m)
		}

		for _, pluginToStart := range args {
			fmt.Println("Starting up", pluginToStart)

			wait.Add(1)

			for _, ch := range input {

				name := pluginToStart
				ch <- launch.StartPlugin{
					Plugin: plugin.Name(name),
					Started: func(config *types.Any) {
						fmt.Println(name, "started.")

						started = append(started, name)
						wait.Done()
					},
					Error: func(config *types.Any, err error) {
						fmt.Println("Error starting", name, "err=", err)
						wait.Done()
					},
				}
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
						log.Infoln("--wait mode: stop scanning.")
						return

					case <-checkNow:
						if m, err := plugins().List(); err == nil {
							now = len(m)
						}
						if now == 0 {
							log.Infoln("--wait mode: scan found no plugins.")
							close(noRunningPlugins)
						}
					}
				}
			}()
		}

		// Here we wait for either wait group to be done or if they are killed out of band.
		select {
		case <-waitDone:
			log.Infoln("All plugins completed. Exiting.")
		case <-noRunningPlugins:
			log.Infoln("Plugins aren't running anymore.  Exiting.")
		}

		for _, monitor := range monitors {
			monitor.Stop()
		}

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
				process.Wait()
				log.Infoln("Process for", n, "exited")
			}

		}

		return nil
	}

	cmd.AddCommand(ls, start, stop)

	return cmd
}
