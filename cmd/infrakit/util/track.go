package util

import (
	"os"
	"strings"
	"time"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/event/instance"
	event_rpc "github.com/docker/infrakit/pkg/rpc/event"
	instance_rpc "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/spf13/cobra"
)

func trackCommand(scp scope.Scope) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "track",
		Short: "Track instances",
	}

	name := cmd.Flags().String("name", "", "Name to use as name of this plugin")
	targets := cmd.Flags().StringSliceP("instance", "n", []string{}, "Instance plugins to track")
	poll := cmd.Flags().DurationP("poll", "i", 3*time.Second, "Polling interval")
	flagTags := cmd.Flags().StringSliceP("tag", "t", []string{}, "Tags to filter instance by")

	cmd.RunE = func(c *cobra.Command, args []string) error {

		if len(args) != 0 {
			cmd.Usage()
			os.Exit(-1)
		}

		tags := map[string]string{}

		for _, tag := range *flagTags {
			kv := strings.SplitN(tag, "=", 2)
			if len(kv) != 2 {
				logger.Warn("bad format tag", "input", tag)
				continue
			}
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			if key != "" && val != "" {
				tags[key] = val
			}
		}

		trackers := map[string]event.Plugin{}

		for _, target := range *targets {

			endpoint, err := scp.Plugins().Find(plugin.Name(target))
			if err != nil {
				return err
			}

			if p, err := instance_rpc.NewClient(plugin.Name(target), endpoint.Address); err == nil {
				trackers[target] = instance.NewTracker(target, p, time.Tick(*poll), tags)
			} else {
				return err
			}
		}

		run.Plugin(plugin.DefaultTransport(*name),
			// As event plugin
			event_rpc.PluginServerWithNames(func() (map[string]event.Plugin, error) {
				return trackers, nil
			}),
		)
		return nil
	}

	return cmd
}
