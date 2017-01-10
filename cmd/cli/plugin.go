package main

import (
	"fmt"
	"sync"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch"
	"github.com/docker/infrakit/pkg/launch/os"
	"github.com/docker/infrakit/pkg/template"
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

		configs := launch.Config([]byte(view))

		parsedRules := []launch.Rule{}
		err = configs.Unmarshal(&parsedRules)
		if err != nil {
			return err
		}

		monitors := []*launch.Monitor{}

		if *osExec {
			exec, err := os.NewLauncher(os.DefaultLogDir())
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

		// now start all the plugins
		for _, plugin := range args {
			fmt.Println("Starting up", plugin)

			wait.Add(1)

			for _, ch := range input {

				name := plugin
				ch <- launch.StartPlugin{
					Plugin: name,
					Started: func(config *launch.Config) {
						fmt.Println(name, "started.")
						wait.Done()
					},
					Error: func(config *launch.Config, err error) {
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

	cmd.AddCommand(ls, start)

	return cmd
}
