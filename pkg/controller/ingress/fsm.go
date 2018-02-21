package ingress

import (
	"fmt"
	"time"

	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/controller/internal"
	"github.com/docker/infrakit/pkg/core"
	"github.com/docker/infrakit/pkg/fsm"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
)

// states
const (
	waiting fsm.Index = iota
	syncing
	follower
)

// signals: note the sync phase is long-running. So we use a start and sync signals to define
// the beginning of the phase.  A start will put the fsm in the syncing state.  And immediately
// we signal the fsm to execute long running work as it transitions to waiting... or if a lead
// signal can interrupt and put it in the follower state. once the task of syncing begins, it must
// run to completion.
const (
	start fsm.Signal = iota
	sync
	lead
	follow
)

var stateMachineSpec, _ = fsm.Define(

	// Waiting state --> waiting until poll time
	fsm.State{
		Index: waiting,
		Transitions: map[fsm.Signal]fsm.Index{
			start:  syncing,
			sync:   waiting,
			lead:   waiting,
			follow: follower,
		},
	},

	// Syncing state --> when syncing backends and routes
	fsm.State{
		Index: syncing,
		Transitions: map[fsm.Signal]fsm.Index{
			sync:   waiting,
			follow: follower,
			lead:   syncing, // if we receive a lead signal, we are still in the same state.
		},
	},

	// Follower state --> when it's not a leader and thus inactive
	fsm.State{
		Index: follower,
		Transitions: map[fsm.Signal]fsm.Index{
			lead:   waiting,
			follow: follower,
		},
	},
)

func (c *managed) defaultBehaviors(spec types.Spec) {

	properties := ingress.Properties{}
	if spec.Properties != nil {
		spec.Properties.Decode(&properties)
	}

	c.properties = properties

	// Set up the functions that actually do the work of fetching routes, backends, etc.
	// get the functions from the properties
	if c.l4s == nil {
		c.l4s = properties.L4Func(c.l4Client)
	}

	if c.routes == nil {
		c.routes = func() (map[ingress.Vhost][]loadbalancer.Route, error) {
			return properties.Routes(c.options)
		}
	}

	if c.groups == nil {
		c.groups = properties.Groups
	}

	if c.instanceIDs == nil {
		c.instanceIDs = properties.InstanceIDs
	}

	if c.healthChecks == nil {
		c.healthChecks = properties.HealthChecks
	}
}

func (c *managed) isLeader() (is bool, err error) {
	check := c.leader()
	if check == nil {
		err = fmt.Errorf("cannot determine leader status")
		return
	}
	is, err = check.IsLeader()
	return
}

func (c *managed) init(in types.Spec) (err error) {
	if c.process != nil {
		panic("this is not allowed")
	}

	c.spec = in
	err = c.spec.Options.Decode(&c.options)
	if err != nil {
		return err
	}

	c.defaultBehaviors(in)

	// Once the state is in the syncing state, we advance the fsm from syncing to waiting by executing
	// work along with the work signal.
	stateMachineSpec.SetAction(syncing, sync,
		func(instance fsm.FSM) error {

			log.Debug("syncing routes and backends")

			err = c.syncRoutesL4()
			if err != nil {
				log.Warn("error syncing routes", "err", err)
			}

			err = c.syncBackends()
			if err != nil {
				log.Warn("error syncing backends", "err", err)
			}

			err = c.syncHealthChecks()
			if err != nil {
				log.Warn("error syncing healthchecks", "err", err)
			}
			return nil
		})

	c.process, err = core.NewProcess(

		func(p *core.Process) (*fsm.Spec, error) {
			return stateMachineSpec, nil
		},

		// stateMachine is the state machine definition for the entire spec and controller states
		// the controller has simple states: start, syncing and waiting.  These states correspond
		// to when the controller is polling and updating or waiting.
		core.ProcessDefinition{
			Spec:        &c.spec,
			Constructor: c.construct,
		},

		core.NewObjects(func(o *types.Object) []interface{} {
			return []interface{}{o.Metadata.Name, o.Metadata.Identity.ID}
		}),

		c.scope,
	)

	if err != nil {
		return
	}

	// start the process with a manual clock which advances only when tick() is called.
	// A clock is required but in this case, we are driving the fsm from the poller,
	// so the manual clock will not be driving any state transitions based on deadlines, etc.
	c.process.Start(fsm.NewClock())

	// create the singleton in the follower state
	c.stateMachine, err = c.process.NewInstance(follower)
	if err != nil {
		return
	}

	if c.ticker == nil {
		interval := c.options.SyncInterval.Duration()
		if interval == 0 {
			interval = ingress.DefaultSyncInterval
		}
		c.ticker = time.Tick(interval)
	}

	// add the poller
	c.poller = internal.Poll(
		func() bool {

			if mustTrue(c.isLeader()) {
				log.Debug("polling", "isLeader", true, "V", debugV)
				c.stateMachine.Signal(lead)
				return true
			}
			c.stateMachine.Signal(follow)
			return false
		},
		func() (err error) {

			c.stateMachine.Signal(start)

			return c.stateMachine.Signal(sync)
		},
		c.ticker,
	)
	return nil
}

func mustTrue(v bool, e error) bool {
	if e != nil {
		return false
	}
	return v
}

func (c *managed) construct(spec types.Spec, properties *types.Any) (*types.Identity, *types.Any, error) {
	state, err := types.AnyValue(c.state())
	return &types.Identity{ID: "ingress-singleton"}, state, err
}

func (c *managed) object() *types.Object {
	return c.process.Object(c.stateMachine)
}
