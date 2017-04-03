package template

import (
	"fmt"
	"io/ioutil"
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
	tflags, _, _, processTemplate := base.TemplateProcessor(plugins)
	cmd := &cobra.Command{
		Use:   "template <url>",
		Short: "Render an infrakit template at given url.  If url is '-', read from stdin",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			url := args[0]
			if url == "-" {
				buff, err := ioutil.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				url = fmt.Sprintf("str://%s", string(buff))
			}

			view, err := processTemplate(url)
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
