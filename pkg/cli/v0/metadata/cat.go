package metadata

import (
	"os"
	gopath "path"
	"strconv"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Cat is the cat command
func Cat(name string, services *cli.Services) *cobra.Command {

	cat := &cobra.Command{
		Use:   "cat",
		Short: "Get metadata entry by path",
	}

	retry := cat.Flags().Duration("retry", 0, "Retry interval (e.g. 1s)")
	timeout := cat.Flags().Duration("timeout", 0, "Timeout")
	errOnTimeout := cat.Flags().Bool("err-on-timeout", false, "Return error on timeout")

	cat.RunE = func(cmd *cobra.Command, args []string) error {

		metadataFunc := scope.MetadataFunc(services.Scope)

		for _, p := range args {

			path := gopath.Join(name, p)

			var value interface{}
			var err error

			if *retry > 0 {
				value, err = metadataFunc(path, *retry, *timeout, *errOnTimeout)
			} else {
				value, err = metadataFunc(path)
			}

			result := value
			if value, is := value.(*types.Any); is && value != nil {
				if s, err := strconv.Unquote(value.String()); err == nil {
					result = s
				}
			}

			if result == nil {
				result = false
			}

			err = services.Output(os.Stdout, result, nil)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return cat
}
