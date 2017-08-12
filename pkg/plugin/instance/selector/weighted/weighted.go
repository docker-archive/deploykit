package weighted

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/instance/selector"
	"github.com/docker/infrakit/pkg/plugin/instance/selector/internal"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "plugin/instance/selector/weighted")

// AffinityArgs contains the arguments specific to this affinity algorithm.  It's in the Args field of selector.Affinity
type AffinityArgs struct {
	Weight uint
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
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
			Name:    "infrakit-instance-selector-weighted",
			Version: "0.1.0",
		},
		URL: "https://github.com/docker/infrakit",
	}
}

// DefaultOptions is the default/example configuration of this plugin
var DefaultOptions = types.AnyValueMust(selector.Options{
	selector.Choice{Name: plugin.Name("zone1"), Affinity: types.AnyValueMust(AffinityArgs{Weight: 50})},
	selector.Choice{Name: plugin.Name("zone2"), Affinity: types.AnyValueMust(AffinityArgs{Weight: 50})},
})

// ExampleProperties returns the properties / config of this plugin
func (p *impl) ExampleProperties() *types.Any {
	return DefaultOptions
}

// SelectOne selects one of the choices given choices and a context (the spec).  It can optionally use the lookup
// to perform queries on the instance plugin to arrive at a decision
func SelectOne(spec instance.Spec, choices []selector.Choice,
	lookup func(selector.Choice) instance.Plugin) (match selector.Choice, err error) {

	distribution := biasesFrom(choices)
	index := roll(distribution)
	if index < 0 {
		err = fmt.Errorf("bad roll with distribution %v", distribution)
		return
	}
	return choices[index], nil
}

func biasesFrom(choices []selector.Choice) []int {
	// Must sort by name of the choice. This will hopefully help in keeping the distribution stationary. Note we
	// not are handling cases when choices are added.
	options := selector.Options(choices)
	sort.Sort(options)

	distribution := []int{}
	for _, c := range options {
		if c.Affinity != nil {
			args := AffinityArgs{}
			if err := c.Affinity.Decode(&args); err == nil {
				distribution = append(distribution, int(args.Weight))
				continue
			}
		}
		log.Warn("no weight found", "choice", c)
		distribution = append(distribution, 0)
	}
	return distribution
}

// The biases distribution must be stationary (not changing). Otherwise long term we will not have the distribution
// of instances across the choices.
func roll(biases []int) int {
	// sum up the biases
	max := 0
	for _, mass := range biases {
		max += mass
	}
	pick := rand.Intn(max) // [0, max)
	sum := 0
	for i, mass := range biases {
		sum += mass
		if pick < sum {
			return i
		}
	}
	return -1
}
