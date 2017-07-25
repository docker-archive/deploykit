package ingress

import (
	"github.com/docker/infrakit/pkg/core"
	"github.com/docker/infrakit/pkg/fsm"
	"github.com/docker/infrakit/pkg/types"
	"golang.org/x/net/context"
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

func optionsFromSpec(spec types.Spec) (Options, error) {
	options := Options{}
	return options, spec.Options.Decode(&options)
}

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

func (c *Controller) start(in types.Spec) (err error) {
	spec := in
	err = spec.Options.Decode(&c.options)
	if err != nil {
		return err
	}

	// Once the state is in the syncing state, we advance the fsm from syncing to waiting by executing
	// work along with the work signal.
	stateMachineSpec.SetAction(syncing, sync,
		func(instance fsm.Instance) error {
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
			Spec:        &spec,
			Constructor: c.construct,
		},

		core.NewObjects(func(o *types.Object) []interface{} {
			return []interface{}{o.Metadata.Name, o.Metadata.Identity.UID}
		}),

		c.plugins,
	)

	if err != nil {
		return
	}

	// create the singleton in the follower state
	c.stateMachine = c.process.Instances().Add(follower)

	stopper := make(chan interface{})

	// add the poller
	c.poller = &Poller{
		interval: c.options.SyncInterval,
		stop:     stopper,
		shouldRun: func() bool {
			if mustTrue(c.leader.IsLeader()) {
				c.stateMachine.Signal(lead)
				return true
			}
			c.stateMachine.Signal(follow)
			return false
		},
		work: func() (err error) {
			c.stateMachine.Signal(start)
			return c.stateMachine.Signal(sync)
		},
	}

	return c.poller.Run(context.Background())
}

func mustTrue(v bool, e error) bool {
	if e != nil {
		return false
	}
	return v
}

func (c *Controller) construct(spec types.Spec, properties *types.Any) (*types.Identity, *types.Any, error) {
	// parse for the spec
	ingressSpec := Spec{}
	err := properties.Decode(&ingressSpec)
	if err != nil {
		return nil, nil, err
	}
	c.spec = ingressSpec
	return &types.Identity{UID: "ingress-singleton"}, properties, nil
}

func (c *Controller) CurrentState() *types.Object {
	return c.process.Object(c.stateMachine)
}
