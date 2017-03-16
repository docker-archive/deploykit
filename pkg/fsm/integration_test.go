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
	creating                 // creating is when the instance is being created
	running                  // running is healthy and running
	down                     // down is not running / unhealthy
	cordoned                 // cordoned is excluded and to be reaped
	terminating              // removed
)

const (
	found     Signal = iota // found is the signal when a resource is to be associated to an instance
	create                  // create provision the instance
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
				found:  allocated,
				create: creating,
			},
			TTL: Expiry{5, create},
		},
		State{
			Index: creating,
			Transitions: map[Signal]Index{
				found: allocated,
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

type machine struct {
	Instance
	id     string // hardware instance id
	config interface{}
}

type cluster struct {
	size  int
	zones [3]*Set
}

func (c *cluster) create(Instance) error {
	return nil
}
func (c *cluster) cordon(Instance) error {
	return nil
}
func (c *cluster) terminate(Instance) error {
	return nil
}

func TestSimpleProvisionFlow(t *testing.T) {

	myCluster := &cluster{
		size: 100,
	}

	actions := map[Signal]Action{
		create:    myCluster.create,
		cordon:    myCluster.cordon,
		terminate: myCluster.terminate,
	}

	spec := simpleProvisionModel(actions)
	require.NotNil(t, spec)

	log.Infoln("Model:", spec)

	clock := Wall(time.Tick(500 * time.Millisecond))

	for i := range myCluster.zones {
		myCluster.zones[i] = NewSet(spec, clock)
	}

	defer func() {
		for i := range myCluster.zones {
			myCluster.zones[i].Stop()
		}
	}()

}
