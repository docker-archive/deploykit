package fsm

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

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
			Actions: map[Signal]Action{
				create: actions[create],
			},
			TTL: Expiry{3, create},
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

type cluster struct {
	size  int
	zones []*Set

	created    int
	cordoned   int
	terminated int

	lock sync.Mutex
}

func (c *cluster) countCreated() int {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.created
}

func (c *cluster) create(FSM) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.created++
	return nil
}
func (c *cluster) cordon(FSM) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.cordoned++
	return nil
}
func (c *cluster) terminate(FSM) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.terminated++
	return nil
}

func (c *cluster) countByState(state Index) int {
	total := 0
	for i := range c.zones {
		total += c.zones[i].CountByState(state)
	}
	return total
}

func TestSimpleProvisionFlow(t *testing.T) {

	total := 30
	zones := 3
	myCluster := &cluster{
		size:  total,
		zones: make([]*Set, zones),
	}

	actions := map[Signal]Action{
		create:    myCluster.create,
		cordon:    myCluster.cordon,
		terminate: myCluster.terminate,
	}

	spec := simpleProvisionModel(actions)
	require.NotNil(t, spec)

	clock := Wall(time.Tick(100 * time.Millisecond)) // per tick
	t.Log("Start the clock")
	clock.Start()

	for i := range myCluster.zones {
		myCluster.zones[i] = NewSet(spec, clock, DefaultOptions(fmt.Sprintf("zone-%d", i)))
	}

	defer func() {
		for i := range myCluster.zones {
			myCluster.zones[i].Stop()
		}
	}()

	t.Log("Creating", myCluster.size, "instances across", len(myCluster.zones), "zones.")

	for i := 0; i < myCluster.size; i++ {
		myCluster.zones[i%zones].Add(specified)
	}

	t.Log("Specified all instances based on spec:")
	require.Equal(t, myCluster.size, func() int {
		total := 0
		for i := range myCluster.zones {
			total += myCluster.zones[i].Size()
		}
		return total
	}())

	// Here we call the infrastructure to list all known instances

	world := []string{} // this is the list of all known instances

	described := make([][]string, zones) // sets of found ids across n zones
	for i := range [10]int{} {
		id := fmt.Sprintf("instance-%d", rand.Intn(total))
		described[i%zones] = append(described[i%zones], id)
		world = append(world, id)
	}

	t.Log("Discover a few instances over 3 zones", described)
	// label / associate with the fsm instances
	associated := 0
	for i := range make([]int, zones) {

		az := myCluster.zones[i]

		total := len(described[i]) // the discovered instances in this zone
		j := 0
		az.ForEachInState(specified,
			func(id ID, state Index, data interface{}) bool {

				require.NoError(t, az.Signal(found, id, described[i][j]))
				associated++

				j++
				return j < total
			})
	}

	// 10 instances have been associated
	require.Equal(t, 10, associated)

	time.Sleep(2 * time.Second)

	require.Equal(t, 10, myCluster.countByState(allocated))
	require.Equal(t, 20, myCluster.countByState(creating))

	// let's say 20 more instances are provisioned now and are coming back when we surveyed the infrastructure:

	// suppose we do a set difference and compute the new ones we haven't seen before.  for each zone, let's
	// put them in a buffered channel
	newIds := map[int]chan string{
		0: make(chan string, 10),
		1: make(chan string, 10),
		2: make(chan string, 10),
	}
	for i := range [20]int{} {
		id := fmt.Sprintf("instance-%d", rand.Intn(total))
		described[i%zones] = append(described[i%zones], id)
		world = append(world, id)

		// push into the buffer channel to be read later when associating ids
		newIds[i%zones] <- id
	}
	// close
	for i := range newIds {
		close(newIds[i])
	}

	require.Equal(t, 30, len(world))

	// now we match the instances again
	// those who are matched are already in allocated state.  so we scan for ones in the creating state

	associated, unassociated := 0, 0
	for i := range myCluster.zones {
		az := myCluster.zones[i]
		az.ForEach(
			func(id ID, s Index, d interface{}) bool {
				switch {
				case s == allocated && d != nil:
					associated++
				case s == creating && d == nil:
					unassociated++

					// get the first available id and attach it
					instanceID := <-newIds[i]

					az.Signal(found, id, instanceID)

					t.Log("associated", id, "to", instanceID)

				}

				return true
			},
		)
	}
	require.Equal(t, 20, unassociated) // all the ones in creating all have no id attached to it.
	require.Equal(t, 10, associated)   // the initial 10 that was first discovered.

	time.Sleep(1 * time.Second) // wait a bit

	require.Equal(t, 30, myCluster.countByState(allocated))

	t.Log("make sure everyone is associated with an instance id from the infrastructure")

	all := 0
	for i := range myCluster.zones {
		az := myCluster.zones[i]
		az.ForEachInState(allocated,
			func(id ID, s Index, d interface{}) bool {
				all++
				require.NotNil(t, d)
				return true
			},
		)
	}

	// Now all instance are provisioned, in allocated state.
	require.Equal(t, 30, all)
}
