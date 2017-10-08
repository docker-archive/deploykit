package metadata

import (
	"os"
	gopath "path"
	"strconv"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Cat is the cat command
func Cat(name string, services *cli.Services) *cobra.Command {

	cat := &cobra.Command{
		Use:   "cat",
		Short: "Get metadata entry by path",
	}

	cat.RunE = func(cmd *cobra.Command, args []string) error {

		metadataPlugin, err := loadPlugin(services.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(metadataPlugin, "metadata plugin not found", "name", name)

		for _, p := range args {

			path := types.PathFromString(gopath.Join(name, p))
			first := path.Index(0)
			if first != nil {

				path = path.Shift(1)

				value, err := metadataPlugin.Get(path)
				if err != nil {
					log.Warn("Cannot metadata cat on plugin", "target", *first, "err", err)
					continue
				}
				if value == nil {
					log.Warn("value is nil")
					continue
				}

				str := value.String()
				if s, err := strconv.Unquote(value.String()); err == nil {
					str = s
				}

				err = services.Output(os.Stdout, str, nil)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	return cat
}
