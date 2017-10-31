package host

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/docker/infrakit/cmd/infrakit/base"

	"github.com/docker/infrakit/pkg/cli"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/host")

func init() {
	base.Register(Command)
}

// Command is the entrypoint
func Command(scope scope.Scope) *cobra.Command {

	///////////////////////////////////////////////////////////////////////////////////
	// remote
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Manage remotes",
	}

	quiet := cmd.PersistentFlags().BoolP("quiet", "q", false, "Print rows without column headers")

	bastionAddr := ""
	user := ""
	set := &cobra.Command{
		Use:   "set <name> <url_list>",
		Short: "Set a remote",
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

			hosts[name] = cli.Remote{
				SSH:       bastionAddr,
				User:      user,
				Endpoints: cli.HostList(urls),
			}

			return hosts.Save()
		},
	}
	set.Flags().StringVar(&bastionAddr, "ssh-addr", bastionAddr, "The public host to tunnel")
	set.Flags().StringVar(&user, "ssh-user", user, "The ssh user")

	remove := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a remote",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) == 0 {
				cmd.Usage()
				os.Exit(1)
			}
			hosts, err := cli.LoadHosts()
			if err != nil {
				return err
			}
			for _, name := range args {
				delete(hosts, name)
			}
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
						fmt.Printf("%-20s\t%-20s\t%-v\n", "HOST", "SSH TUNNEL", "URL LIST")
					}

					h := []string{}
					for name := range hosts {
						h = append(h, name)
					}

					sort.Strings(h)

					for _, name := range h {
						remote := hosts[name]
						fmt.Printf("%-20v\t%-20v\t%-v\n", name, remote.SSH, remote.Endpoints)
					}
					return nil
				})
		},
	}
	list.Flags().AddFlagSet(outputFlags)

	cmd.AddCommand(set, remove, list, current)
	return cmd
}
