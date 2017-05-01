package metadata

import (
	"fmt"
	"os"
	"strconv"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/rpc/client"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/metadata")

func init() {
	base.Register(Command)
}

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

// Command is the entry point to this module
func Command(plugins func() discovery.Plugins) *cobra.Command {

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
	quick := ls.Flags().BoolP("quick", "q", false, "True to turn off headers, etc.")

	ls.RunE = func(c *cobra.Command, args []string) error {
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
			first := path.Index(0)

			targets := []string{} // target plugins to query

			// Check all the plugins -- scanning via discovery
			if err := forPlugin(plugins,
				func(name string, mp metadata.Plugin) error {
					if p == "." || (first != nil && name == *first) {
						targets = append(targets, name)
					}
					return nil
				}); err != nil {
				return err
			}

			for j, target := range targets {

				nodes := []types.Path{} // the result set to print

				match, err := getPlugin(plugins, target)
				if err != nil {
					return err
				}

				if p == "." {
					if *all {
						allPaths, err := listAll(match, path.Shift(1))
						if err != nil {
							log.Warn("Cannot metadata ls on plugin", "target", target, "err", err)
						}
						for _, c := range allPaths {
							nodes = append(nodes, types.PathFromString(target).Join(c))
						}
					} else {
						for _, t := range targets {
							nodes = append(nodes, types.PathFromString(t))
						}
					}

				} else {
					if *all {
						allPaths, err := listAll(match, path.Shift(1))
						if err != nil {
							log.Warn("Cannot metadata ls on plugin", "target", target, "err", err)
						}
						for _, c := range allPaths {
							nodes = append(nodes, types.PathFromString(target).Join(c))
						}
					} else {
						children, err := match.List(path.Shift(1))
						if err != nil {
							log.Warn("Cannot metadata ls on plugin", "target", target, "err", err)
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
			}

		}
		return nil
	}

	catFlags, catOutput := base.Output()
	cat := &cobra.Command{
		Use:   "cat",
		Short: "Get metadata entry by path",
		RunE: func(c *cobra.Command, args []string) error {

			for _, p := range args {

				path := types.PathFromString(p)
				first := path.Index(0)
				if first != nil {
					match, err := getPlugin(plugins, *first)
					if err != nil {
						return err
					}

					if match == nil {
						return fmt.Errorf("plugin not found:%v", *first)
					}

					if path.Len() == 1 {
						fmt.Printf("%v\n", match != nil)
					} else {
						value, err := match.Get(path.Shift(1))
						if err == nil {
							if value != nil {
								str := value.String()
								if s, err := strconv.Unquote(value.String()); err == nil {
									str = s
								}

								catOutput(os.Stdout, str)
							}
						} else {
							log.Warn("Cannot metadata cat on plugin", "target", *first, "err", err)
						}
					}
				}
			}
			return nil
		},
	}
	cat.Flags().AddFlagSet(catFlags)

	cmd.AddCommand(ls, cat)

	return cmd
}
