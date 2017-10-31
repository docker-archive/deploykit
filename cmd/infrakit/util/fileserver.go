package util

import (
	"net/http"
	"os"

	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/spf13/cobra"
)

func fileServerCommand(scp scope.Scope) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fileserver <path>",
		Short: "Fileserver over http",
	}

	listen := cmd.Flags().StringP("listen", "l", ":8080", "Listening port")

	cmd.RunE = func(c *cobra.Command, args []string) error {

		if len(args) != 1 {
			c.Usage()
			os.Exit(-1)
		}

		logger.Info("Starting file server", "listen", *listen)

		rootFS := args[0]
		handler := http.FileServer(http.Dir(rootFS))
		return http.ListenAndServe(*listen, handler)
	}

	return cmd
}
