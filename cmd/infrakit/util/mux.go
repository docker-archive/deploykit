package util

import (
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/rpc/mux"
	"github.com/spf13/cobra"
)

func muxCommand(plugins func() discovery.Plugins) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mux",
		Short: "API mux service",
	}

	// http://www.speedguide.net/port.php?port=24864 - unassigned by IANA
	listen := cmd.Flags().StringP("listen", "l", ":24864", "Listening port")
	autoStop := cmd.Flags().BoolP("auto-stop", "a", false, "True to stop when no plugins are running")
	interval := cmd.Flags().DurationP("scan", "s", 1*time.Minute, "Scan interval to check for plugins")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		logger.Info("Starting mux server", "listen", *listen)
		server, err := mux.NewServer(*listen, plugins)
		if err != nil {
			return err
		}
		defer server.Stop()

		block := make(chan struct{})
		// If the sockets are gone, then we can safely exit.
		go func() {
			checkNow := time.Tick(*interval)
			for {
				select {
				case <-server.Wait():
					logger.Info("server stopped")
					close(block)
					return

				case <-checkNow:
					if m, err := plugins().List(); err == nil {
						if len(m) == 0 && *autoStop {
							logger.Info("scan found no plugins.")
							close(block)
							return
						}
					}

				}
			}
		}()

		<-block

		return nil
	}

	return cmd
}
