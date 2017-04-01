package template

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/cmd/cli/base"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/template")

func init() {
	base.Register(Command)
}

// Command is the entrypoint
func Command(plugins func() discovery.Plugins) *cobra.Command {

	///////////////////////////////////////////////////////////////////////////////////
	// template
	tflags, processTemplate := base.TemplateProcessor(plugins)
	cmd := &cobra.Command{
		Use:   "template <url>",
		Short: "Render an infrakit template at given url.  If url is '-', read from stdin",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 0 {
				cmd.Usage()
				os.Exit(1)
			}

			log.Debug("reading template", "url", args[0])
			view, err := base.ReadFromStdinIfElse(
				func() bool { return args[0] == "-" },
				func() (string, error) { return processTemplate(args[0]) },
			)
			if err != nil {
				return err
			}
			fmt.Print(view)
			return nil

		},
	}
	cmd.Flags().AddFlagSet(tflags)

	return cmd
}
