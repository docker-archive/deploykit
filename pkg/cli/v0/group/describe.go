package group

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/spf13/cobra"
)

// Describe returns the Describe command
func Describe(name string, services *cli.Services) *cobra.Command {

	describe := &cobra.Command{
		Use:   "describe <group ID>",
		Short: "Describe a group. Returns a list of members",
	}

	quiet := describe.Flags().BoolP("quiet", "q", false, "Print rows without column headers")
	describe.RunE = func(cmd *cobra.Command, args []string) error {

		pluginName := plugin.Name(name)
		_, gid := pluginName.GetLookupAndType()
		if gid == "" {
			if len(args) < 1 {
				cmd.Usage()
				os.Exit(1)
			} else {
				gid = args[0]
			}
		}

		groupPlugin, err := LoadPlugin(services.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

		groupID := group.ID(gid)

		desc, err := groupPlugin.DescribeGroup(groupID)
		if err != nil {
			return err
		}

		return services.Output(os.Stdout, desc,
			func(io.Writer, interface{}) error {
				if !*quiet {
					fmt.Printf("%-30s\t%-30s\t%-s\n", "ID", "LOGICAL", "TAGS")
				}
				for _, d := range desc.Instances {
					logical := "  -   "
					if d.LogicalID != nil {
						logical = string(*d.LogicalID)
					}

					printTags := []string{}
					for k, v := range d.Tags {
						printTags = append(printTags, fmt.Sprintf("%s=%s", k, v))
					}
					sort.Strings(printTags)

					fmt.Printf("%-30s\t%-30s\t%-s\n", d.ID, logical, strings.Join(printTags, ","))
				}
				return nil
			})
	}
	describe.Flags().AddFlagSet(services.OutputFlags)
	return describe
}
