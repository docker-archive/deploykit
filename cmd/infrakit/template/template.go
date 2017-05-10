package template

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/infrakit/cmd/infrakit/base"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/template")

func init() {
	base.Register(Command)
}

// Command is the entrypoint
func Command(plugins func() discovery.Plugins) *cobra.Command {

	templateFlags, _, _, processTemplate := base.TemplateProcessor(plugins)

	///////////////////////////////////////////////////////////////////////////////////
	// template
	cmd := &cobra.Command{
		Use:   "template <url>",
		Short: "Render an infrakit template at given url.  If url is '-', read from stdin",
	}

	outputFile := cmd.PersistentFlags().StringP("output", "o", "", "Output filename")
	cmd.Flags().AddFlagSet(templateFlags)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {

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

		if *outputFile != "" {
			return ioutil.WriteFile(*outputFile, []byte(view), 0644)
		}

		fmt.Print(view)
		return nil
	}

	format := &cobra.Command{
		Use:   "format json|yaml",
		Short: "Converts stdin to different format",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			in, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return err
			}

			buff := []byte(in)
			switch strings.ToLower(args[0]) {

			case "json":
				buff, err = yaml.YAMLToJSON(buff)
			case "yaml":
				buff, err = yaml.JSONToYAML(buff)
			default:
				err = fmt.Errorf("unknown format %s", args[0])
			}

			if err != nil {
				return err
			}

			if *outputFile != "" {
				return ioutil.WriteFile(*outputFile, buff, 0644)
			}

			fmt.Print(string(buff))
			return nil

		},
	}
	cmd.AddCommand(format)

	return cmd
}
