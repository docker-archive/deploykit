package main

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	metadata_template "github.com/docker/infrakit/pkg/plugin/metadata/template"
	"github.com/docker/infrakit/pkg/rpc/client"
	event_rpc "github.com/docker/infrakit/pkg/rpc/event"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

func getEventPlugin(plugins func() discovery.Plugins, name string) (found event.Plugin, err error) {
	err = forEventPlugins(plugins, func(n string, p event.Plugin) error {
		if n == name {
			found = p
		}
		return nil
	})
	return
}

func forEventPlugins(plugins func() discovery.Plugins, do func(string, event.Plugin) error) error {
	all, err := plugins().List()
	if err != nil {
		return err
	}
	for name, endpoint := range all {
		rpcClient, err := client.New(endpoint.Address, event.InterfaceSpec)
		if err != nil {
			continue
		}
		if err := do(name, event_rpc.Adapt(rpcClient)); err != nil {
			return err
		}
	}
	return nil
}

func listAllTopics(m event.Plugin, path types.Path) ([]types.Path, error) {
	result := []types.Path{}
	nodes, err := m.List(path)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		c := path.JoinString(n)
		more, err := listAllTopics(m, c)
		if err != nil {
			return nil, err
		}
		if len(more) == 0 {
			result = append(result, c)
		}
		result = append(result, more...)
	}
	return result, nil
}

func eventCommand(plugins func() discovery.Plugins) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "event",
		Short: "Access event exposed by infrakit plugins",
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List all event entries",
	}

	long := ls.Flags().BoolP("long", "l", false, "Print full path")
	all := ls.Flags().BoolP("all", "a", false, "Find all under the paths given")
	quick := ls.Flags().BoolP("quick", "q", false, "True to turn off headers, etc.")

	ls.RunE = func(c *cobra.Command, args []string) error {
		paths := []string{"."}

		// All implies long
		if *all {
			*long = true
		}

		if len(args) > 0 {
			paths = []string{}
			for _, p := range args {
				if p == "/" {
					// TODO(chungers) -- this is a 'local' infrakit ensemble.
					// Absolute paths will come in a multi-cluster / federated model.
					return fmt.Errorf("No absolute path")
				}
				paths = append(paths, p)
			}
		}

		// For each of the paths the user provided in the argv:
		for i, p := range paths {

			path := types.PathFromString(p)
			first := path.Index(0)

			targets := map[string]event.Plugin{} // target plugins to query
			// Check all the plugins -- scanning via discovery
			if err := forEventPlugins(plugins,
				func(name string, mp event.Plugin) error {
					if path.Dot() || (first != nil && name == *first) {
						targets[name] = mp
					}
					return nil
				}); err != nil {
				return err
			}

			j := 0
			for target, match := range targets {

				nodes := []types.Path{} // the result set to print

				if *all {
					allPaths, err := listAllTopics(match, path.Shift(1))
					if err != nil {
						log.Warningln("Cannot event ls on plugin", target, "err=", err)
					}
					for _, c := range allPaths {
						nodes = append(nodes, types.PathFromString(target).Join(c))
					}
				} else {
					if p == "." {
						for t := range targets {
							nodes = append(nodes, types.PathFromString(t))
						}
					} else {
						children, err := match.List(path.Shift(1))
						if err != nil {
							log.Warningln("Cannot event ls on plugin", target, "err=", err)
						}
						for _, c := range children {
							nodes = append(nodes, path.JoinString(c))
						}
					}
				}

				if p == "." && !*all {
					// special case of showing the top level plugin namespaces
					if i > 0 && !*quick {
						fmt.Println()
					}
					for _, l := range nodes {
						if *long {
							fmt.Println(l)
						} else {
							fmt.Println(l.Rel(path))
						}
					}
					break
				}

				if len(targets) > 1 {
					if j > 0 && !*quick {
						fmt.Println()
					}
					fmt.Printf("%s:\n", target)
				}
				if *long && !*quick {
					fmt.Printf("total %d:\n", len(nodes))
				}
				for _, l := range nodes {
					if *long {
						fmt.Println(l)
					} else {
						fmt.Println(l.Rel(path))
					}
				}

				j++
			}

		}
		return nil
	}

	globals := []string{}
	templateURL := "str://{{.}}"
	tail := &cobra.Command{
		Use:   "tail",
		Short: "tail a stream by topic",
		RunE: func(c *cobra.Command, args []string) error {

			log.Infof("Using %v for rendering view.", templateURL)
			engine, err := template.NewTemplate(templateURL, template.Options{
				SocketDir: local.Dir(),
			})
			if err != nil {
				return err
			}
			for _, global := range globals {
				kv := strings.Split(global, "=")
				if len(kv) != 2 {
					continue
				}
				key := strings.Trim(kv[0], " \t\n")
				val := strings.Trim(kv[1], " \t\n")
				if key != "" && val != "" {
					engine.Global(key, val)
				}
			}
			engine.WithFunctions(func() []template.Function {
				return []template.Function{
					{
						Name: "metadata",
						Description: []string{
							"Metadata function takes a path of the form \"plugin_name/path/to/data\"",
							"and calls GET on the plugin with the path \"path/to/data\".",
							"It's identical to the CLI command infrakit metadata cat ...",
						},
						Func: metadata_template.MetadataFunc(plugins),
					},
				}
			})

			if len(args) == 0 {
				args = []string{"."}
			}

			topics := []types.Path{}
			for _, a := range args {
				p := types.PathFromString(a).Clean()
				if p.Valid() {
					topics = append(topics, p)
				}
			}

			if len(topics) == 0 {
				return nil
			}

			targets := map[string]event.Plugin{} // target plugins to query
			for _, topic := range topics {
				first := topic.Index(0)
				// Check all the plugins -- scanning via discovery
				if err := forEventPlugins(plugins,
					func(name string, mp event.Plugin) error {
						if topic.Dot() || (first != nil && name == *first) {
							targets[name] = mp
						}
						return nil
					}); err != nil {
					return err
				}
			}

			collector := make(chan *event.Event)
			done := make(chan int)
			running := 0

			// now subscribe to each topic
			for _, topic := range topics {

				target := *topic.Index(0)
				eventTopic := topic.Shift(1)

				plugin := targets[target]
				if plugin == nil {
					return fmt.Errorf("no client:%v", target)
				}

				client, is := plugin.(event.Subscriber)
				if !is {
					return fmt.Errorf("not a subscriber: %s, %v", target, plugin)
				}

				log.Infoln("Subscribing to", eventTopic)

				stream, err := client.SubscribeOn(eventTopic)
				if err != nil {
					return fmt.Errorf("cannot subscribe: %s, err=%v", topic, err)
				}

				go func() {
					defer func() { done <- -1 }()

					for {
						select {
						case evt, ok := <-stream:
							if !ok {
								log.Infoln("Server disconnected -- topic=", topic)
								return
							}

							// Scope the topic
							evt.Topic = types.PathFromString(target).Join(evt.Topic)
							collector <- evt
						}
					}
				}()

				running++
			}

		loop:
			for {
				select {
				case v := <-done:
					running += v
					if running == 0 {
						break loop
					}

				case evt, ok := <-collector:
					if !ok {
						log.Infoln("Server disconnected.")
						break loop
					}
					buff, err := engine.Render(evt)
					if err != nil {
						log.Warningln("error rendering view: %v", err)
					} else {
						fmt.Println(buff)
					}
				}
			}

			return nil
		},
	}
	tail.Flags().StringVar(&templateURL, "url", templateURL, "URL for the template")

	cmd.AddCommand(ls, tail)

	return cmd
}
