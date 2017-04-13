package main

import (
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	event_rpc "github.com/docker/infrakit/pkg/rpc/event"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

const (
	timerType = event.Type("timer")
)

type timer struct {
	stop   chan struct{}
	topics map[string]interface{}
}

func (t *timer) getEndpoint() interface{} {
	return "to-be-implemented:the url endpoint"
}

func (t *timer) init() *timer {
	t.topics = map[string]interface{}{}

	for _, topic := range types.PathFromStrings(
		"msec/100",
		"msec/500",
		"sec/1",
		"sec/5",
		"sec/30",
		"min/1",
		"min/5",
		"min/30",
		"hr/1",
	) {
		types.Put(topic, t.getEndpoint, t.topics)
	}

	return t
}

// List returns the nodes under the given topic
func (t *timer) List(topic types.Path) ([]string, error) {
	return types.List(topic, t.topics), nil
}

// PublishOn sets the channel to publish on
func (t *timer) PublishOn(c chan<- *event.Event) {
	go func() {
		defer close(c)

		log.Infoln("Timer starting publish on channel:", c)

		tick100msec := time.Tick(100 * time.Millisecond)
		tick500msec := time.Tick(500 * time.Millisecond)
		tick1sec := time.Tick(1 * time.Second)
		tick5sec := time.Tick(5 * time.Second)
		tick30sec := time.Tick(30 * time.Second)
		tick1min := time.Tick(1 * time.Minute)
		tick5min := time.Tick(5 * time.Minute)
		tick30min := time.Tick(30 * time.Minute)
		tick1hr := time.Tick(1 * time.Hour)

		for {
			select {
			case <-t.stop:
				return

			case <-tick100msec:
				c <- event.Event{
					Type: timerType,
					ID:   "100ms",
				}.Init().Now().WithTopic("msec/100").WithDataMust(time.Duration(100 * time.Millisecond))

			case <-tick500msec:
				c <- event.Event{
					Type: timerType,
					ID:   "500ms",
				}.Init().Now().WithTopic("msec/500").WithDataMust(time.Duration(500 * time.Millisecond))

			case <-tick1sec:
				c <- event.Event{
					Type: timerType,
					ID:   "1sec",
				}.Init().Now().WithTopic("sec/1").WithDataMust(time.Duration(1 * time.Second))

			case <-tick5sec:
				c <- event.Event{
					Type: timerType,
					ID:   "5s",
				}.Init().Now().WithTopic("sec/5").WithDataMust(time.Duration(5 * time.Second))

			case <-tick30sec:
				c <- event.Event{
					Type: timerType,
					ID:   "30s",
				}.Init().Now().WithTopic("sec/30").WithDataMust(time.Duration(30 * time.Second))

			case <-tick1min:
				c <- event.Event{
					Type: timerType,
					ID:   "1m",
				}.Init().Now().WithTopic("min/1").WithDataMust(time.Duration(1 * time.Minute))

			case <-tick5min:
				c <- event.Event{
					Type: timerType,
					ID:   "5m",
				}.Init().Now().WithTopic("min/5").WithDataMust(time.Duration(5 * time.Minute))

			case <-tick30min:
				c <- event.Event{
					Type: timerType,
					ID:   "30m",
				}.Init().Now().WithTopic("min/30").WithDataMust(time.Duration(30 * time.Minute))

			case <-tick1hr:
				c <- event.Event{
					Type: timerType,
					ID:   "1h",
				}.Init().Now().WithTopic("hr/1").WithDataMust(time.Duration(1 * time.Hour))

			}
		}
	}()
}

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Timer event plugin",
	}

	name := cmd.Flags().String("name", "event-time", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")

	cmd.RunE = func(c *cobra.Command, args []string) error {

		cli.SetLogLevel(*logLevel)

		stop := make(chan struct{})

		// For metadata -- queries for current time
		timeQueries := map[string]interface{}{}
		types.Put(types.PathFromString("now/nano"),
			func() interface{} {
				return time.Now().UnixNano()
			},
			timeQueries)
		types.Put(types.PathFromString("now/sec"),
			func() interface{} {
				return time.Now().Unix()
			},
			timeQueries)

		// For events
		timerEvents := (&timer{stop: stop}).init()

		cli.RunPlugin(*name,

			// As metadata plugin
			metadata_rpc.PluginServer(metadata_plugin.NewPluginFromData(map[string]interface{}{
				"version":    cli.Version,
				"revision":   cli.Revision,
				"implements": event.InterfaceSpec,
			})).WithTypes(
				map[string]metadata.Plugin{
					"time": metadata_plugin.NewPluginFromData(timeQueries),
				}),

			// As event plugin
			event_rpc.PluginServerWithTypes(
				map[string]event.Plugin{
					"timer": timerEvents,
				}))

		close(stop)

		return nil
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
