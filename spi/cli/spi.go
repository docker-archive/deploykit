package cli

import "github.com/spf13/cobra"

// DriverCLI is the interface by which driver implementations are exposed to the CLI.
type DriverCLI interface {
	Command() *cobra.Command
}
