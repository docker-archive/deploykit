package flavor

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Healthy returns the healthy command
func Healthy(name string, services *cli.Services) *cobra.Command {

	flavorPropertiesURL := ""

	healthy := &cobra.Command{
		Use:   "healthy id...",
		Short: "Healthy checks the healthy of the instances by ids given",
	}
	healthy.Flags().String("properties", "", "Properties of the flavor plugin, a url")

	tags := healthy.Flags().StringSlice("tags", []string{}, "Tags to filter")
	asLogicalID := false
	healthy.Flags().BoolVarP(&asLogicalID, "logical-id", "l", asLogicalID, "Args are logical IDs")
	healthy.Flags().AddFlagSet(services.ProcessTemplateFlags)

	healthy.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}

		flavorPlugin, err := services.Scope.Flavor(name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(flavorPlugin, "instance plugin not found", "name", name)

		flavorProperties, err := services.ProcessTemplate(flavorPropertiesURL)
		if err != nil {
			return err
		}

		filter := map[string]string{}
		for _, t := range *tags {
			p := strings.Split(t, "=")
			if len(p) == 2 {
				filter[p[0]] = p[1]
			} else {
				filter[p[0]] = ""
			}
		}

		for _, arg := range args {

			desc := instance.Description{}
			if len(filter) > 0 {
				desc.Tags = filter
			}

			if asLogicalID {
				logical := instance.LogicalID(arg)
				desc.LogicalID = &logical
			} else {
				desc.ID = instance.ID(arg)
			}

			healthy, err := flavorPlugin.Healthy(types.AnyString(flavorProperties), desc)
			if err == nil {
				fmt.Printf("%v\n", healthy)
			}
		}

		return err
	}
	return healthy
}
