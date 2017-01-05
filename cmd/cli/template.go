package main

import (
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/template"
	"github.com/spf13/cobra"
)

func templateCommand(plugins func() discovery.Plugins) *cobra.Command {

	templateURL := ""
	contextURL := ""
	stdin := false

	cmd := &cobra.Command{
		Use:   "template",
		Short: "Render an infrakit template",
		RunE: func(cmd *cobra.Command, args []string) error {

			opt := template.Options{
				SocketDir: discovery.Dir(),
			}
			useStdin := templateURL == "" && stdin
			if useStdin {
				log.Infoln("Using stdin for reading template")

				if contextURL == "" {
					pwd, err := os.Getwd()
					if err != nil {
						pwd = os.Getenv("PWD")
					}
					if pwd != "" {
						contextURL = "file://localhost" + pwd + "/"
					}
				}

				engine, err := template.NewTemplateFromReader(os.Stdin, contextURL, opt)
				if err != nil {
					return err
				}

				view, err := engine.Render(nil)
				if err != nil {
					return err
				}

				fmt.Print(view)
				return nil

			} else {

				log.Infof("Using %v for reading template\n", templateURL)
				engine, err := template.NewTemplate(templateURL, opt)
				if err != nil {
					return err
				}

				view, err := engine.Render(nil)
				if err != nil {
					return err
				}

				fmt.Print(view)
				return nil
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&templateURL, "url", "", "URL for the template")
	cmd.Flags().StringVar(&contextURL, "root", "", "Parent context URL for including templates.  All relative paths used in 'include' will be relative to this root.")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "True to read template from stdin")

	return cmd
}
