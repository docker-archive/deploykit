package time

import (
	"time"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "time"
)

var (
	log = logutil.New("module", "run/v0/time")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

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

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Metadata: metadata_plugin.NewPluginFromData(timeQueries),
		run.Event: func() (map[string]event.Plugin, error) {
			return map[string]event.Plugin{
				"streams": timerEvents,
			}, nil
		},
	}
	onStop = func() { close(stop) }
	return
}

const (
	timeType = event.Type("time")
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

	for _, topic := range types.PathsFromStrings(
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

		log.Info("Time starting publish", "channel", c)

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
					Type: timeType,
					ID:   "100ms",
				}.Init().Now().WithTopic("msec/100").WithDataMust(time.Duration(100 * time.Millisecond))

			case <-tick500msec:
				c <- event.Event{
					Type: timeType,
					ID:   "500ms",
				}.Init().Now().WithTopic("msec/500").WithDataMust(time.Duration(500 * time.Millisecond))

			case <-tick1sec:
				c <- event.Event{
					Type: timeType,
					ID:   "1sec",
				}.Init().Now().WithTopic("sec/1").WithDataMust(time.Duration(1 * time.Second))

			case <-tick5sec:
				c <- event.Event{
					Type: timeType,
					ID:   "5s",
				}.Init().Now().WithTopic("sec/5").WithDataMust(time.Duration(5 * time.Second))

			case <-tick30sec:
				c <- event.Event{
					Type: timeType,
					ID:   "30s",
				}.Init().Now().WithTopic("sec/30").WithDataMust(time.Duration(30 * time.Second))

			case <-tick1min:
				c <- event.Event{
					Type: timeType,
					ID:   "1m",
				}.Init().Now().WithTopic("min/1").WithDataMust(time.Duration(1 * time.Minute))

			case <-tick5min:
				c <- event.Event{
					Type: timeType,
					ID:   "5m",
				}.Init().Now().WithTopic("min/5").WithDataMust(time.Duration(5 * time.Minute))

			case <-tick30min:
				c <- event.Event{
					Type: timeType,
					ID:   "30m",
				}.Init().Now().WithTopic("min/30").WithDataMust(time.Duration(30 * time.Minute))

			case <-tick1hr:
				c <- event.Event{
					Type: timeType,
					ID:   "1h",
				}.Init().Now().WithTopic("hr/1").WithDataMust(time.Duration(1 * time.Hour))

			}
		}
	}()
}
