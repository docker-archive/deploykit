package spread

import (
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/instance/selector"
	"github.com/docker/infrakit/pkg/plugin/instance/selector/internal"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "plugin/instance/selector/spread")

// AffinityArgs is the struct that contains parameters that are important for this algorithm to work.
// Because this algorithm dependes on querying the plugins for the instances, we need to know what
// labels to use for filtering.  The labels should match those that are generated by the group controller
// so that we can at least have the same representation in this as how the group controller sees the world.
// TODO - we can reference the group this plugin is to be used with.  However, we'd have to introduce a
// query mechanism for this plugin to query for information about the group. Alternatively, we can also
// have the labels be dynamically populated by the launcher of the plugin, which would at that time have
// information on the labels to use.
type AffinityArgs struct {
	// Labels are the labels to use to filter the query to the instance plugin associated with this Choice.
	Labels map[string]string
}

type impl struct {
	instance.Plugin
}

// NewPlugin returns an instance plugin that implements this algorithm
func NewPlugin(plugins func() discovery.Plugins, choices selector.Options) instance.Plugin {
	return &impl{
		Plugin: &internal.Base{
			Plugins:    plugins,
			Choices:    choices,
			SelectFunc: SelectOne,
		},
	}
}

// Info returns a vendor specific name and version
func (p *impl) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-selector-spread",
			Version: "0.1.0",
		},
		URL: "https://github.com/docker/infrakit",
	}
}

// DefaultOptions is the default/example configuration of this plugin
var DefaultOptions = types.AnyValueMust(selector.Options{
	selector.Choice{Name: plugin.Name("zone1")},
	selector.Choice{Name: plugin.Name("zone2")},
})

// ExampleProperties returns the properties / config of this plugin
func (p *impl) ExampleProperties() *types.Any {
	return DefaultOptions
}

func getLabels(choice selector.Choice) map[string]string {
	if choice.Affinity == nil {
		return nil
	}
	args := AffinityArgs{}
	err := choice.Affinity.Decode(&args)
	if err != nil {
		return nil
	}
	return args.Labels
}

// SelectOne selects one of the choices given choices and a context (the spec).  It can optionally use the lookup
// to perform queries on the instance plugin to arrive at a decision.  In the case of spread affinity, we pick
// the instance plugin that reports the least number of instances.
func SelectOne(spec instance.Spec, choices []selector.Choice,
	lookup func(selector.Choice) instance.Plugin) (match selector.Choice, err error) {

	// Pick the one with the fewest number of instances
	min := 1<<63 - 1
	for _, choice := range choices {

		p := lookup(choice)
		if p == nil {
			log.Warn("cannot get instance", "choice", choice)
			continue
		}

		list, err := p.DescribeInstances(getLabels(choice), false)
		if err != nil {
			log.Warn("error querying instance plugin", "choice", choice, "plugin", p)
			continue
		}

		if len(list) < min {
			match = choice
			min = len(list)
		}
	}
	return
}
