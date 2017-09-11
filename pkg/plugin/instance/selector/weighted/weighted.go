package weighted

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin/instance/selector"
	"github.com/docker/infrakit/pkg/plugin/instance/selector/internal"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
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
	base := &internal.Base{
		Plugins:    plugins,
		Choices:    choices,
		SelectFunc: SelectOne,
	}
	return &impl{
		Plugin: base.Init(),
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

// SelectOne selects one of the choices given choices and a context (the spec).  It can optionally use the lookup
// to perform queries on the instance plugin to arrive at a decision
func SelectOne(spec instance.Spec, choices []selector.Choice,
	lookup func(selector.Choice) instance.Plugin) (match selector.Choice, err error) {

	distribution := biasesFrom(choices)
	pick := roll(distribution)
	index := bin(distribution, pick)
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
	return rand.Intn(max) // [0, max)
}

// The biases distribution must be stationary (not changing). Otherwise long term we will not have the distribution
// of instances across the choices.
func bin(biases []int, pick int) int {
	sum := 0
	for i, mass := range biases {
		sum += mass
		if pick < sum {
			return i
		}
	}
	return -1
}
