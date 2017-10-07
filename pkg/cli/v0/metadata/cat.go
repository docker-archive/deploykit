package metadata

import (
	"fmt"
	"os"
	"strconv"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Cat returns the Cat command
func Cat(name string, services *cli.Services,
	loader func(discovery.Plugins, string) (metadata.Plugin, error)) *cobra.Command {

	cat := &cobra.Command{
		Use:   "cat",
		Short: "Get metadata entry by path",
	}

	cat.RunE = func(cmd *cobra.Command, args []string) error {

		metadataPlugin, err := loader(services.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(metadataPlugin, "metadata plugin not found", "name", name)

		for _, p := range args {

			path := types.PathFromString(p)
			first := path.Index(0)
			if first != nil {

				if path.Len() == 1 {
					fmt.Printf("%v\n", metadataPlugin != nil)
				} else {
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
		}
		return nil
	}
	return cat
}
