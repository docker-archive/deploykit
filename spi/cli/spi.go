package cli

import "github.com/spf13/cobra"

type DriverCli interface {
	Command() *cobra.Command
}
