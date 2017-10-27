package metadata

import (
	"fmt"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Ls returns the Ls command
func Ls(name string, services *cli.Services) *cobra.Command {

	mls := &cobra.Command{
		Use:   "ls",
		Short: "List metadata",
	}

	long := mls.Flags().BoolP("long", "l", false, "Print full path")
	all := mls.Flags().BoolP("all", "a", false, "Find all under the paths given")
	quick := mls.Flags().BoolP("quick", "q", false, "True to turn off headers, etc.")

	mls.RunE = func(cmd *cobra.Command, args []string) error {

		metadataPlugin, err := loadPlugin(services.Plugins(), name)
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
					fmt.Println(l.Rel(types.PathFromString(p)))
				}
				break
			}

			if *long && !*quick {
				fmt.Printf("total %d:\n", len(nodes))
			}
			for _, l := range nodes {
				fmt.Println(l.Rel(types.PathFromString(p)))
			}

		}
		return nil
	}
	return mls
}

func listAll(m metadata.Plugin, path types.Path) ([]types.Path, error) {
	if m == nil {
		return nil, fmt.Errorf("no plugin")
	}
	result := []types.Path{}
	nodes, err := m.List(path)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		c := path.JoinString(n)
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
