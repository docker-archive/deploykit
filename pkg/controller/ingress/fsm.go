package ingress

import (
	"time"

	"github.com/docker/infrakit/pkg/controller"
	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/core"
	"github.com/docker/infrakit/pkg/fsm"
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

func (c *Controller) init(in types.Spec) (err error) {
	if c.process != nil {
		return nil // no op
	}

	spec := in
	err = spec.Options.Decode(&c.options)
	if err != nil {
		return err
	}

	// Once the state is in the syncing state, we advance the fsm from syncing to waiting by executing
	// work along with the work signal.
	stateMachineSpec.SetAction(syncing, sync,
		func(instance fsm.Instance) error {
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

	// start the process with a manual clock which advances only when tick() is called.
	// A clock is required but in this case, we are driving the fsm from the poller,
	// so the manual clock will not be driving any state transitions based on deadlines, etc.
	c.process.Start(fsm.NewClock())

	// create the singleton in the follower state
	c.stateMachine, err = c.process.NewInstance(follower)
	if err != nil {
		return
	}

	//c.stateMachine = c.process.Instances().Add(follower)

	if c.ticker == nil && c.options.SyncInterval > 0 {
		c.ticker = time.Tick(c.options.SyncInterval)
	}

	// add the poller
	c.poller = controller.Poll(
		func() bool {
			if mustTrue(c.leader.IsLeader()) {
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

	// c.poller = &Poller{
	// 	ticker: c.ticker,
	// 	stop:   make(chan interface{}),
	// 	shouldRun: func() bool {
	// 		if mustTrue(c.leader.IsLeader()) {
	// 			c.stateMachine.Signal(lead)
	// 			return true
	// 		}
	// 		c.stateMachine.Signal(follow)
	// 		return false
	// 	},
	// 	work: func() (err error) {
	// 		c.stateMachine.Signal(start)
	// 		return c.stateMachine.Signal(sync)
	// 	},
	// }
	return nil
}

func mustTrue(v bool, e error) bool {
	if e != nil {
		return false
	}
	return v
}

func (c *Controller) construct(spec types.Spec, properties *types.Any) (*types.Identity, *types.Any, error) {
	// parse for the spec
	ingressSpec := ingress.Properties{}
	err := properties.Decode(&ingressSpec)
	if err != nil {
		return nil, nil, err
	}
	c.spec = ingressSpec
	state, err := types.AnyValue(c.state())
	return &types.Identity{UID: "ingress-singleton"}, state, err
}

func (c *Controller) object() *types.Object {
	return c.process.Object(c.stateMachine)
}
