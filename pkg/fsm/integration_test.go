package fsm

import (
	"fmt"
	"math/rand"
	"sync"
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

type machine struct {
	Instance
	id     string // hardware instance id
	config interface{}
}

type cluster struct {
	size     int
	zones    []*Set
	machines [][]*machine

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

func (c *cluster) create(Instance) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.created++
	return nil
}
func (c *cluster) cordon(Instance) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.cordoned++
	return nil
}
func (c *cluster) terminate(Instance) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.terminated++
	return nil
}

func TestSimpleProvisionFlow(t *testing.T) {

	total := 30
	zones := 3
	myCluster := &cluster{
		size:     total,
		zones:    make([]*Set, zones),
		machines: make([][]*machine, zones),
	}

	actions := map[Signal]Action{
		create:    myCluster.create,
		cordon:    myCluster.cordon,
		terminate: myCluster.terminate,
	}

	spec := simpleProvisionModel(actions)
	require.NotNil(t, spec)

	clock := Wall(time.Tick(100 * time.Millisecond)) // per tick
	log.Infoln("Start the clock")
	clock.Start()

	for i := range myCluster.zones {
		myCluster.zones[i] = NewSet(spec, clock)
	}

	defer func() {
		for i := range myCluster.zones {
			myCluster.zones[i].Stop()
		}
	}()

	log.Infoln("Creating", myCluster.size, "instances across", len(myCluster.zones), "zones.")

	for i := 0; i < myCluster.size; i++ {
		instance := myCluster.zones[i%zones].Add(specified)
		m := &machine{Instance: instance}
		myCluster.machines[i%zones] = append(myCluster.machines[i%zones], m)
	}

	log.Infoln("Specified all instances based on spec:")
	require.Equal(t, myCluster.size, func() int {
		total := 0
		for i := range myCluster.zones {
			total += myCluster.zones[i].Size()
		}
		return total
	}())

	// Here we call the infrastructure to list all known instances

	described := make([][]string, zones) // sets of found ids across n zones
	for i := range [10]int{} {
		described[i%zones] = append(described[i%zones], fmt.Sprintf("instance-%d", rand.Intn(total)))
	}
	log.Infoln("Discover a few instances over 3 zones", described)

	// label / associate with the fsm instances
	for i := range make([]int, zones) {

		az := myCluster.zones[i]
		machines := myCluster.machines[i]

		total := len(described[i]) // the discovered instances in this zone
		j := 0
		az.ForEachInstance(func(id ID, state Index) bool {
			if state == specified {

				require.NoError(t, az.Signal(found, id))
				machines[id].id = described[i][j]

				log.Infoln("associated", described[i][j], "to", machines[id])

				j++
			}
			return j < total
		})
	}

	start := time.Now()
	for {
		count := myCluster.countCreated()
		log.Infoln("We should be creating some instances:", count, "create calls made.")

		if count == 20 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	log.Info("elapsed:", time.Now().Sub(start))
}
