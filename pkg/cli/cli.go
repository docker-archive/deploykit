package cli // import "github.com/docker/infrakit/pkg/cli"

import (
	"github.com/spf13/cobra"
)

// Modules provides access to CLI module discovery
type Modules interface {

	// List returns a list of preconfigured commands
	List() ([]*cobra.Command, error)
}
