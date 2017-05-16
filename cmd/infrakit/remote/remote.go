package host

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/host")

func init() {
	base.Register(Command)
}

// Command is the entrypoint
func Command(plugins func() discovery.Plugins) *cobra.Command {

	///////////////////////////////////////////////////////////////////////////////////
	// remote
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Manage remotes",
	}

	quiet := cmd.PersistentFlags().BoolP("quiet", "q", false, "Print rows without column headers")

	add := &cobra.Command{
		Use:   "add <name> <url_list>",
		Short: "Add a remote",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 2 {
				cmd.Usage()
				os.Exit(1)
			}

			name := args[0]
			urls := args[1]

			hosts, err := cli.LoadHosts()
			if err != nil {
				return err
			}

			hosts[name] = cli.HostList(urls)

			return hosts.Save()
		},
	}
	remove := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a remote",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			name := args[0]

			hosts, err := cli.LoadHosts()
			if err != nil {
				return err
			}

			delete(hosts, name)

			return hosts.Save()
		},
	}
	current := &cobra.Command{
		Use:   "current",
		Short: "Show current remote (set by INFRAKIT_HOST)",
		RunE: func(cmd *cobra.Command, args []string) error {
			val := os.Getenv("INFRAKIT_HOST")
			fmt.Println(val)
			return nil
		},
	}

	outputFlags, output := cli.Output()
	list := &cobra.Command{
		Use:   "ls",
		Short: "List remotes",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 0 {
				cmd.Usage()
				os.Exit(1)
			}

			hosts, err := cli.LoadHosts()
			if err != nil {
				return err
			}

			return output(os.Stdout, hosts,
				func(io.Writer, interface{}) error {
					if !*quiet {
						fmt.Printf("%-30s\t%-30s\n", "HOST", "URL LIST")
					}

					h := []string{}
					for host := range hosts {
						h = append(h, host)
					}

					sort.Strings(h)

					for _, host := range h {
						urls := hosts[host]
						fmt.Printf("%-30v\t%-30v\n", host, urls)
					}
					return nil
				})
		},
	}
	list.Flags().AddFlagSet(outputFlags)

	cmd.AddCommand(add, remove, list, current)
	return cmd
}
