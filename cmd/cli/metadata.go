package main

import (
	"fmt"
	"sort"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/rpc/client"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/spf13/cobra"
)

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
		metadata := metadata_rpc.Adapt(rpcClient)
		if err := do(name, metadata); err != nil {
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

	ls.RunE = func(c *cobra.Command, args []string) error {
		paths := []string{"/"}

		if len(args) > 0 {
			paths = args
		}

		nodes := []string{}
		for _, path := range paths {

			p := path

			err := forPlugin(plugins, func(name string, mp metadata.Plugin) error {

				if p != "/" && name != p {
					return nil
				}

				children, err := mp.List(metadata_plugin.Path(p))
				if err != nil {
					log.Warningln("Cannot metadata ls on plugin", name, "err=", err)
					return nil // do not stop
				}

				// Children is unqualified name so we need to prepend with the name of the plugin.
				for _, c := range children {
					nodes = append(nodes, metadata_plugin.String(metadata_plugin.PathFromStrings(name, c)))
				}
				return nil
			})

			if err != nil {
				return err
			}

		}

		sort.Strings(nodes)

		fmt.Println("There are", len(nodes), "entries:")
		for _, l := range nodes {
			fmt.Println(l)
		}
		return nil
	}

	cmd.AddCommand(ls)

	return cmd
}
