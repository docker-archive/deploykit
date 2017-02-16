package main

import (
	"fmt"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/rpc/client"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/spf13/cobra"
)

func getPlugin(plugins func() discovery.Plugins, name string) (found metadata.Plugin, err error) {
	err = forPlugin(plugins, func(n string, p metadata.Plugin) error {
		if n == name {
			found = p
		}
		return nil
	})
	return
}

func forPlugin(plugins func() discovery.Plugins, do func(string, metadata.Plugin) error) error {
	all, err := plugins().List()
	if err != nil {
		return err
	}
	for name, endpoint := range all {
		rpcClient, err := client.New(endpoint.Address, metadata.InterfaceSpec)
		if err != nil {
			continue
		}
		if err := do(name, metadata_rpc.Adapt(rpcClient)); err != nil {
			return err
		}
	}
	return nil
}

func listAll(m metadata.Plugin, path metadata.Path) ([]metadata.Path, error) {
	if m == nil {
		return nil, fmt.Errorf("no plugin")
	}
	result := []metadata.Path{}
	nodes, err := m.List(path)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		c := path.Join(n)
		more, err := listAll(m, c)
		if err != nil {
			return nil, err
		}
		if len(more) == 0 {
			result = append(result, c)
		}
		for _, pp := range more {
			result = append(result, pp)
		}
	}
	return result, nil
}

func metadataCommand(plugins func() discovery.Plugins) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Access metadata exposed by infrakit plugins",
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List all metadata entries",
	}

	long := ls.Flags().BoolP("long", "l", false, "Print full path")
	all := ls.Flags().BoolP("all", "a", false, "Find all under the paths given")

	ls.RunE = func(c *cobra.Command, args []string) error {
		paths := []string{"."}

		// All implies long
		if *all {
			*long = true
		}

		if len(args) > 0 {
			paths = args
		}

		for _, p := range paths {

			if p == "/" {
				// TODO(chungers) -- this is a 'local' infrakit ensemble.
				// Absolute paths will come in a multi-cluster / federated model.
				return fmt.Errorf("No absolute path")
			}

			nodes := []metadata.Path{}
			path := metadata_plugin.Path(p)
			targets := []string{} // target plugins to query

			if p == "." {

				allMetadataPlugins := []string{} // name of all plugins implementing metadata spi

				// Check all the plugins -- scanning via discovery
				if err := forPlugin(plugins, func(name string, mp metadata.Plugin) error {
					allMetadataPlugins = append(allMetadataPlugins, name)
					if !*all {
						// append the result only if this is not listing all nodes
						// the all nodes case will run over all the targets so no need to
						// include the child here.
						nodes = append(nodes, path.Join(name))
					}
					return nil
				}); err != nil {
					return err
				}

				if *all {
					targets = allMetadataPlugins
				}

			} else if first := path.Index(0); first != nil {
				targets = []string{*first}
			}

			for _, target := range targets {

				match, err := getPlugin(plugins, target)
				if err != nil {
					return err
				}

				if *all {
					allPaths, err := listAll(match, path.Shift(1))
					if err != nil {
						log.Warningln("Cannot metadata ls on plugin", target, "err=", err)
					}
					for _, c := range allPaths {
						nodes = append(nodes, path.Join(target).Sub(c))
					}
				} else {
					children, err := match.List(path.Shift(1))
					if err != nil {
						log.Warningln("Cannot metadata ls on plugin", target, "err=", err)
					}
					for _, c := range children {
						nodes = append(nodes, path.Join(c))
					}
				}
			}

			for _, l := range nodes {
				if *long {
					fmt.Println(metadata_plugin.String(l))
				} else {
					fmt.Println(l.Base())
				}
			}
		}
		return nil
	}

	cat := &cobra.Command{
		Use:   "cat",
		Short: "Get metadata entry by path",
		RunE: func(c *cobra.Command, args []string) error {

			for _, p := range args {

				path := metadata_plugin.Path(p)
				first := path.Index(0)
				if first != nil {
					match, err := getPlugin(plugins, *first)
					if err != nil {
						return err
					}

					value, err := match.Get(path.Shift(1))
					if err == nil {
						if value != nil {
							str := value.String()
							if s, err := strconv.Unquote(value.String()); err == nil {
								str = s
							}
							fmt.Println(str)
						}

					} else {
						log.Warningln("Cannot metadata cat on plugin", *first, "err=", err)
					}
				}
			}
			return nil
		},
	}

	cmd.AddCommand(ls, cat)

	return cmd
}
