package group

import (
	"strconv"
	"time"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/group"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin. Used for command line identification
	Kind = "group"

	// LookupName is the name used to look up the object via discovery
	//LookupName = "group-stateless"

	// EnvPollInterval is the frequency for polling
	EnvPollInterval = "INFRAKIT_GROUP_POLL_INTERVAL"

	// EnvMaxParallelNum sets the max parallelism for creating instances
	EnvMaxParallelNum = "INFRAKIT_GROUP_MAX_PARALLEL_NUM"
)

var log = logutil.New("module", "run/group")

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the group controller.
type Options struct {
	// PollInterval is the frequency for syncing the state
	PollInterval types.Duration

	// MaxParallelNum is the max number of parallel instance operation. Default =0 (no limit)
	MaxParallelNum uint

	// PollIntervalGroupSpec polls for group spec at this interval to update the metadata paths
	PollIntervalGroupSpec types.Duration

	// PollIntervalGroupDetail polls for group details at this interval to update the metadata paths
	PollIntervalGroupDetail types.Duration
}

func mustParseUint(s string) uint {
	v, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return uint(v)
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	PollInterval:            types.MustParseDuration(local.Getenv(EnvPollInterval, "10s")),
	MaxParallelNum:          mustParseUint(local.Getenv(EnvMaxParallelNum, "0")),
	PollIntervalGroupSpec:   types.MustParseDuration(local.Getenv(EnvPollInterval, "10s")),
	PollIntervalGroupDetail: types.MustParseDuration(local.Getenv(EnvPollInterval, "10s")),
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	log.Debug("Starting group", "name", name, "configs", config)

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	instancePluginLookup := func(n plugin.Name) (instance.Plugin, error) {
		return scope.Instance(n.String())
	}

	flavorPluginLookup := func(n plugin.Name) (flavor.Plugin, error) {
		return scope.Flavor(n.String())
	}

	groupPlugin := group.NewGroupPlugin(instancePluginLookup, flavorPluginLookup,
		options.PollInterval.Duration(), options.MaxParallelNum)

	// Start a poller to load the snapshot and make that available as metadata
	updateSnapshot := make(chan func(map[string]interface{}))
	stopSnapshot := make(chan struct{})
	go func() {
		tick := time.Tick(options.PollIntervalGroupSpec.Duration())
		tick2 := time.Tick(options.PollIntervalGroupDetail.Duration())
		for {
			select {
			case <-tick:
				// load the specs for the groups
				snapshot := map[string]interface{}{}
				if specs, err := groupPlugin.InspectGroups(); err == nil {
					for _, spec := range specs {
						snapshot[string(spec.ID)] = spec
					}
				} else {
					snapshot["err"] = err
				}

				updateSnapshot <- func(view map[string]interface{}) {
					types.Put([]string{"specs"}, snapshot, view)
				}

			case <-tick2:
				snapshot := map[string]interface{}{}
				// describe the groups and expose info as metadata
				if specs, err := groupPlugin.InspectGroups(); err == nil {
					for _, spec := range specs {
						if description, err := groupPlugin.DescribeGroup(spec.ID); err == nil {
							snapshot[string(spec.ID)] = description
						} else {
							snapshot[string(spec.ID)] = err
						}
					}
				} else {
					snapshot["err"] = err
				}

				updateSnapshot <- func(view map[string]interface{}) {
					types.Put([]string{"groups"}, snapshot, view)
				}

			case <-stopSnapshot:
				log.Info("Snapshot updater stopped")
				return
			}
		}
	}()

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Metadata: metadata_plugin.NewPluginFromChannel(updateSnapshot),
		run.Group:    groupPlugin,
	}
	onStop = func() {
		close(stopSnapshot)
	}
	return
}
