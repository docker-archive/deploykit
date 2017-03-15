package fsm

import (
	"testing"
	"time"

	log "github.com/golang/glog"
	"github.com/stretchr/testify/require"
)

const (
	specified   Index = iota // first specified by the user's spec
	allocated                // allocated is matched to an external resource found
	running                  // running is healthy and running
	down                     // down is not running / unhealthy
	cordoned                 // cordoned is excluded and to be reaped
	terminating              // removed
)

const (
	found     Signal = iota // found is the signal when a resource is to be associated to an instance
	healthy                 // healthy is when the resource is determined healthy
	unhealthy               // unhealthy is when the resource is not healthy
	cordon                  // cordon marks the instance as off limits / use
	terminate               // terminate the instance
)

func simpleProvisionModel(actions map[Signal]Action) *Spec {

	spec, err := Define(
		State{
			Index: specified,
			Transitions: map[Signal]Index{
				found: allocated,
			},
			Actions: map[Signal]Action{
				found: actions[found],
			},
		},
		State{
			Index: allocated,
			Transitions: map[Signal]Index{
				healthy:   running,
				unhealthy: down,
				terminate: terminating,
			},
			Actions: map[Signal]Action{
				terminate: actions[terminate],
			},
		},
		State{
			Index: running,
			Transitions: map[Signal]Index{
				unhealthy: down,
				terminate: terminating,
			},
			Actions: map[Signal]Action{
				terminate: actions[terminate],
			},
		},
		State{
			Index: down,
			Transitions: map[Signal]Index{
				healthy:   running,
				unhealthy: down,
				cordon:    cordoned,
				terminate: terminating,
			},
			Actions: map[Signal]Action{
				cordon:    actions[cordon],
				terminate: actions[terminate],
			},
			TTL: Expiry{5, cordon},
		},
		State{
			Index: cordoned,
			Transitions: map[Signal]Index{
				terminate: terminating,
			},
		},
		State{
			Index: terminating,
		},
	)

	if err != nil {
		panic(err)
	}
	return spec
}

func TestSimpleProvisionFlow(t *testing.T) {

	actions := map[Signal]Action{}

	actions[cordon] = func(Instance) {}
	actions[terminate] = func(Instance) {}
	actions[found] = func(Instance) {}

	spec := simpleProvisionModel(actions)
	require.NotNil(t, spec)

	log.Infoln("Model:", spec)

	clock := Wall(time.Tick(500 * time.Millisecond))

	azs := make([]*Set, 3)

	for i := range azs {
		azs[i] = NewSet(spec, clock)
	}

	defer func() {
		for i := range azs {
			azs[i].Stop()
		}
	}()
}
