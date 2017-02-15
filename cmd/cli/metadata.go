package main

import (
	"fmt"
	"sort"
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

	ls.RunE = func(c *cobra.Command, args []string) error {
		paths := []string{"."}

		if len(args) > 0 {
			paths = args
		}

		for _, p := range paths {

			if p == "/" {
				return fmt.Errorf("No absolute path")
			}

			nodes := []string{}
			all := p == "."
			path := metadata_plugin.Path(p)

			if all {
				if err := forPlugin(plugins, func(name string, mp metadata.Plugin) error {
					nodes = append(nodes, name)
					return nil
				}); err != nil {
					return err
				}
			} else {
				first := path.Index(0)
				if first != nil {
					match, err := getPlugin(plugins, *first)
					if err != nil {
						return err
					}

					children, err := match.List(path.Shift(1))
					if err == nil {
						nodes = append(nodes, children...)
					} else {
						log.Warningln("Cannot metadata ls on plugin", *first, "err=", err)
					}
				}
			}

			sort.Strings(nodes)
			for _, l := range nodes {
				if *long {
					fmt.Printf("%s/%s\n", metadata_plugin.String(path), l)
				} else {
					fmt.Println(l)
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
