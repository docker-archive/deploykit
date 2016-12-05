package cli

import (
	"fmt"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/spf13/cobra"
)

// MetaCommand creates a cobra Command that prints build version information.
func MetaCommand(pluginObj func() interface{}) *cobra.Command {
	return &cobra.Command{
		Use:   "meta",
		Short: "print plugin metadata",
		RunE: func(cmd *cobra.Command, args []string) error {

			if pluginObj() == nil {
				return fmt.Errorf("no plugin")
			}

			if informer, is := pluginObj().(plugin.Informer); is {

				m, err := informer.GetMeta()
				if err != nil {
					return err
				}

				fmt.Println(m)
				return nil
			}
			return fmt.Errorf("no metadata available")
		},
	}
}
