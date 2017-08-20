package loadbalancer

import (
	"fmt"
	"io"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Backends returns the describe command
func Backends(name string, services *cli.Services) *cobra.Command {
	backends := &cobra.Command{
		Use:   "backends",
		Short: "List all backends",
	}
	backends.Flags().AddFlagSet(services.OutputFlags)
	backends.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		l4, err := Load(services.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(l4, "L4 not found", "name", name)

		list, err := l4.Backends()
		if err != nil {
			return err
		}

		return services.Output(os.Stdout, list,
			func(w io.Writer, v interface{}) error {

				for _, r := range list {

					buff, err := types.AnyValueMust(r).MarshalYAML()
					if err != nil {
						return err
					}
					fmt.Printf("%v\n", string(buff))
				}
				return nil
			})
	}
	return backends
}
