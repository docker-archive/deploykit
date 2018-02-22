package resource

import (
	"context"

	"github.com/docker/infrakit/pkg/controller/internal"
	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
)

type collection struct {
	*internal.Collection

	properties resource.Properties
	options    resource.Options

	instancePlugins map[string]*internal.InstanceObserver
}

func newCollection(scope scope.Scope, leader func() stack.Leadership,
	options resource.Options) (internal.Managed, error) {

	if err := options.Validate(context.Background()); err != nil {
		return nil, err
	}

	base, err := internal.NewCollection(scope, leader)
	if err != nil {
		return nil, err
	}
	c := &collection{
		Collection:      base,
		options:         options,
		instancePlugins: map[string]*internal.InstanceObserver{},
	}

	// set the behaviors
	base.StartFunc = c.start
	base.StopFunc = c.stop
	base.UpdateSpecFunc = c.updateSpec
	return c, nil
}

func (c *collection) start(ctx context.Context) {
	log.Info("starting")
}

func (c *collection) stop() error {
	log.Info("stop")
	return nil
}

func (c *collection) updateSpec(spec types.Spec) (err error) {

	defer log.Debug("updateSpec", "spec", spec, "err", err)

	// parse input, then select the model to use
	properties := resource.Properties{}
	err = spec.Properties.Decode(&properties)
	if err != nil {
		return
	}

	ctx := context.Background()
	if err = properties.Validate(ctx); err != nil {
		return
	}

	c.properties = properties

	return
}
