package tiered

import (
	"fmt"

	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin/instance/selector"
	"github.com/docker/infrakit/pkg/plugin/instance/selector/internal"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "plugin/instance/selector/tiered")

type impl struct {
	instance.Plugin
}

// NewPlugin returns an instance plugin that implements this algorithm
func NewPlugin(plugins func() discovery.Plugins, choices selector.Options) instance.Plugin {
	base := &internal.Base{
		Plugins: plugins,
		Choices: choices,
	}
	i := &impl{
		Plugin: base.Init(),
	}
	return i
}

// Info returns a vendor specific name and version
func (p *impl) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-selector-tiered",
			Version: "0.1.0",
		},
		URL: "https://github.com/docker/infrakit",
	}
}

// Provision creates a new instance based on the spec. This overrides the base Provision
func (p *impl) Provision(spec instance.Spec) (*instance.ID, error) {
	cprops := map[string]*types.Any{}
	err := spec.Properties.Decode(&cprops)
	if err != nil {
		return nil, err
	}

	// visit the choices one by one
	base, is := p.Plugin.(*internal.Base)
	if !is {
		panic("Not implemented with internal.Base")
	}

	var provisioned *instance.ID

	err = base.VisitChoices(
		func(c selector.Choice, p instance.Plugin) (bool, error) {

			var properties *types.Any
			found, ok := cprops[string(c.Name)]

			if !ok {
				return false, fmt.Errorf("no config for %v", c.Name)
			}
			properties = found

			if properties == nil {
				return false, fmt.Errorf("no config for %v", c.Name)
			}

			copy := spec
			copy.Properties = properties

			id, err := p.Provision(copy)
			if err == nil && id != nil {
				// successfully provisioned the instance. stop here.
				idCopy := *id
				provisioned = &idCopy
				return false, nil
			}
			return true, nil
		})

	if provisioned == nil {
		err = fmt.Errorf("cannot provision instance")
	}

	return provisioned, err
}
