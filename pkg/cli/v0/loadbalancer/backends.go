package loadbalancer

import (
	"fmt"
	"io"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/cobra"
)

// Backends returns the describe command
func Backends(name string, services *cli.Services) *cobra.Command {
	backends := &cobra.Command{
		Use:   "backends",
		Short: "Loadbalancer backends",
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List loadbalancer backends",
	}
	register := &cobra.Command{
		Use:   "add <instance.ID> ...",
		Short: "Register backends []instance.ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			l4, err := services.Scope.L4(name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(l4, "L4 not found", "name", name)

			ids := []instance.ID{}
			for _, a := range args {
				ids = append(ids, instance.ID(a))
			}

			res, err := l4.RegisterBackends(ids)
			fmt.Println(res)
			return err
		},
	}

	deregister := &cobra.Command{
		Use:   "rm <instance.ID> ...",
		Short: "Deregister backends []instance.ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			l4, err := services.Scope.L4(name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(l4, "L4 not found", "name", name)

			ids := []instance.ID{}
			for _, a := range args {
				ids = append(ids, instance.ID(a))
			}

			res, err := l4.DeregisterBackends(ids)
			fmt.Println(res)
			return err
		},
	}
	backends.AddCommand(ls, register, deregister)

	ls.Flags().AddFlagSet(services.OutputFlags)
	ls.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		l4, err := services.Scope.L4(name)
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

				fmt.Printf("%-20v\n", "INSTANCE ID")
				for _, r := range list {
					fmt.Printf("%-20v\n", r)
				}
				return nil
			})
	}
	return backends
}
