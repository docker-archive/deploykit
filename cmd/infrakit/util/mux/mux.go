package mux

import (
	"net/url"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/leader"
	"github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/rpc/mux"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/spf13/cobra"
)

var logger = log.New("module", "util/mux")

type config struct {
	listen       *string
	autoStop     *bool
	interval     *time.Duration
	pollInterval *time.Duration
	location     *string
	plugins      func() discovery.Plugins
	poller       *leader.Poller
	store        leader.Store
}

// Command returns the cobra command
func Command(scp scope.Scope) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "mux",
		Short: "API mux service",
	}

	// http://www.speedguide.net/port.php?port=24864 - unassigned by IANA
	listen := cmd.PersistentFlags().StringP("listen", "l", ":24864", "Listening port")
	autoStop := cmd.PersistentFlags().BoolP("auto-stop", "a", false, "True to stop when no plugins are running")
	interval := cmd.PersistentFlags().DurationP("scan", "s", 1*time.Minute, "Scan interval to check for plugins")
	pollInterval := cmd.PersistentFlags().DurationP("poll-interval", "p", 5*time.Second, "Leader polling interval")
	locateURL := cmd.Flags().StringP("locate-url", "u", "", "Locate URL of this node, eg. http://public_ip:24864")

	config := &config{
		location:     locateURL,
		plugins:      scp.Plugins,
		listen:       listen,
		autoStop:     autoStop,
		interval:     interval,
		pollInterval: pollInterval,
	}

	cmd.RunE = func(c *cobra.Command, args []string) error {
		return runMux(config)
	}

	cmd.AddCommand(
		osEnvironment(config),
		swarmEnvironment(config),
		etcdEnvironment(config),
	)

	return cmd
}

func runMux(config *config) error {

	var leadership <-chan leader.Leadership

	if config.store != nil && config.poller != nil {
		logger.Info("Starting leader poller")
		defer config.poller.Stop()
		l, err := config.poller.Start()
		if err != nil {
			return err
		}
		leadership = l
	}

	advertise, err := url.Parse(*config.location)
	if err != nil {
		return err
	}
	logger.Info("Starting mux server", "listen", *config.listen)
	server, err := mux.NewServer(*config.listen, advertise.Host, config.plugins,
		mux.Options{
			Leadership: leadership,
			Registry:   config.store,
		})
	if err != nil {
		return err
	}
	defer server.Stop()

	block := make(chan struct{})
	// If the sockets are gone, then we can safely exit.
	go func() {
		checkNow := time.Tick(*config.interval)
		for {
			select {
			case <-server.Wait():
				logger.Info("server stopped")
				close(block)
				return

			case <-checkNow:
				if m, err := config.plugins().List(); err == nil {
					if len(m) == 0 && *config.autoStop {
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
