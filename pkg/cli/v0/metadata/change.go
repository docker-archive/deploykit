package metadata

import (
	"fmt"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Change returns the Change command
func Change(name string, services *cli.Services) *cobra.Command {

	ls := &cobra.Command{
		Use:   "change",
		Short: "Update metadata",
	}

	long := ls.Flags().BoolP("long", "l", false, "Print full path")
	all := ls.Flags().BoolP("all", "a", false, "Find all under the paths given")
	quick := ls.Flags().BoolP("quick", "q", false, "True to turn off headers, etc.")

	ls.RunE = func(cmd *cobra.Command, args []string) error {

		metadataPlugin, err := loadPluginUpdatable(services.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(metadataPlugin, "metadata plugin not found", "name", name)

		paths := []string{"."}

		// All implies long
		if *all {
			*long = true
		}

		if len(args) > 0 {
			paths = args
		}

		for i, p := range paths {

			if p == "/" {
				// TODO(chungers) -- this is a 'local' infrakit ensemble.
				// Absolute paths will come in a multi-cluster / federated model.
				return fmt.Errorf("No absolute path")
			}

			path := types.PathFromString(p)

			nodes := []types.Path{} // the result set to print

			if *all {
				allPaths, err := listAll(metadataPlugin, path)
				if err != nil {
					log.Warn("Cannot metadata ls on plugin", "name", name, "err", err)
				}
				for _, c := range allPaths {
					nodes = append(nodes, c)
				}
			} else {
				children, err := metadataPlugin.List(path)
				if err != nil {
					log.Warn("Cannot metadata ls on plugin", "name", name, "err", err)
				}
				for _, c := range children {
					nodes = append(nodes, path.JoinString(c))
				}
			}

			if p == "." && !*all {
				// special case of showing the top level plugin namespaces
				if i > 0 && !*quick {
					fmt.Println()
				}
				for _, l := range nodes {
					if *long {
						fmt.Println(l.Rel(path))
					} else {
						fmt.Println(l.Rel(path))
					}
				}
				break
			}

			if *long && !*quick {
				fmt.Printf("total %d:\n", len(nodes))
			}
			for _, l := range nodes {
				fmt.Println(l.Rel(path))
			}

		}
		return nil
	}
	return ls
}
