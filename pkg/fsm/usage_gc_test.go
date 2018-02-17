package fsm

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	dockerNodeReady Signal = iota
	dockerNodeDown
	dockerNodeGone
	instanceOK
	instanceGone
	timeout
	reap
)

const (
	start                  Index = iota
	matchedInstance              // has vm information, waiting to match to docker_node
	matchedDockerNode            // has docker_node information, waiting to match to vm
	swarmNode                    // has matching docker_node and vm information
	swarmNodeReady               // ready as swarm node
	swarmNodeDown                // unavailable as swarm node
	pendingInstanceDestroy       // vm needs to be removedInstance (instance destroy)
	removedInstance              // instance is deleted
	done                         // terminal
)

type gc struct {
	ch chan FSM
	op string
}

func (gc gc) do(i FSM) error {
	fmt.Println(gc.op, i)
	gc.ch <- i
	return nil
}

func TestSwarmEntities(t *testing.T) {

	pollInterval := 100 * time.Millisecond

	noData := Tick(10)
	dockerNodeJoin := Tick(5)
	waitDescribeInstances := Tick(5)
	waitBeforeInstanceDestroy := Tick(3)
	waitBeforeReprovision := Tick(10) // wait before we reprovision a new instance to fix a Down node
	waitBeforeCleanup := Tick(10)

	// actions
	dockerNodeRm := &gc{
		ch: make(chan FSM, 1),
		op: "dockerNodeRm",
	}

	instanceDestroy := &gc{
		ch: make(chan FSM, 1),
		op: "instanceDestroy",
	}

	model, err := Define(
		State{
			Index: start,
			TTL:   Expiry{noData, timeout},
			Transitions: map[Signal]Index{
				dockerNodeReady: matchedDockerNode,
				dockerNodeDown:  matchedDockerNode,
				instanceOK:      matchedInstance,
				timeout:         removedInstance, // nothing happened... cleanup
			},
		},
		State{
			Index: matchedInstance,
			TTL:   Expiry{dockerNodeJoin, dockerNodeGone},
			Transitions: map[Signal]Index{
				dockerNodeReady: swarmNode,
				dockerNodeDown:  swarmNode,
				dockerNodeGone:  pendingInstanceDestroy,
				instanceGone:    removedInstance,
			},
			Actions: map[Signal]Action{
				instanceGone: instanceDestroy.do,
			},
		},
		State{
			Index: pendingInstanceDestroy,
			TTL:   Expiry{waitBeforeInstanceDestroy, reap},
			Transitions: map[Signal]Index{
				dockerNodeReady: swarmNode, // late joiner
				dockerNodeDown:  swarmNode,
				instanceGone:    removedInstance,
				reap:            removedInstance,
			},
			Actions: map[Signal]Action{
				instanceGone: instanceDestroy.do,
				reap:         instanceDestroy.do,
			},
		},
		State{
			Index: matchedDockerNode,
			TTL:   Expiry{waitDescribeInstances, instanceGone},
			Transitions: map[Signal]Index{
				instanceOK:     swarmNode,
				instanceGone:   removedInstance,
				dockerNodeGone: removedInstance, // could be docker rm'd out of band
			},
			Actions: map[Signal]Action{
				instanceGone: dockerNodeRm.do,
			},
		},
		State{
			Index: swarmNode,
			Transitions: map[Signal]Index{
				dockerNodeReady: swarmNodeReady,
				dockerNodeDown:  swarmNodeDown,
				dockerNodeGone:  matchedInstance,
				instanceGone:    matchedDockerNode,
			},
		},
		State{
			Index: swarmNodeReady,
			Transitions: map[Signal]Index{
				dockerNodeDown: swarmNodeDown,
				dockerNodeGone: matchedInstance,
				instanceGone:   matchedDockerNode,
			},
		},
		State{
			Index: swarmNodeDown,
			TTL:   Expiry{waitBeforeReprovision, dockerNodeGone},
			Transitions: map[Signal]Index{
				dockerNodeReady: swarmNodeReady,
				dockerNodeGone:  pendingInstanceDestroy,
				instanceGone:    matchedDockerNode,
			},
		},
		State{
			Index: removedInstance, // after we removed the instance, we can still have unmatched node
			TTL:   Expiry{waitBeforeCleanup, timeout},
			Transitions: map[Signal]Index{
				dockerNodeDown: done,
				timeout:        done,
			},
			Actions: map[Signal]Action{
				dockerNodeDown: dockerNodeRm.do,
			},
		},
		State{
			Index: done, // deleted state is terminal. this will be garbage collected
		},
	)
	require.NoError(t, err)

	model.SetStateNames(map[Index]string{
		start:                  "START",
		matchedInstance:        "FOUND_INSTANCE",
		matchedDockerNode:      "FOUND_DOCKER_NODE",
		swarmNode:              "SWARM_NODE",
		swarmNodeReady:         "READY",
		swarmNodeDown:          "DOWN",
		pendingInstanceDestroy: "PENDING_INSTNACE_DESTROY",
		removedInstance:        "REMOVED_INSTANCE",
		done:                   "DONE",
	}).SetSignalNames(map[Signal]string{
		dockerNodeReady: "docker-node-ready",
		dockerNodeDown:  "docker-node-down",
		dockerNodeGone:  "docker-node-gone",
		instanceOK:      "instance-ok",
		instanceGone:    "instance-gone",
		timeout:         "timeout",
		reap:            "reap",
	})

	clock := Wall(time.Tick(pollInterval))
	clock.Start()

	t.Log(model)

	set := NewSet(model, clock, DefaultOptions("test"))
	defer func() {
		set.Stop()
		clock.Stop()
	}()

	group := map[string]FSM{
		"case1": set.Add(start),
	}

	{
		// case 1 - orphaned ex-leader node, a unmatched "Down" node where instance was removed and deletion observed.
		// add new instance in start state
		// we found the node via docker node ls and it's in ready state
		require.NoError(t, group["case1"].Signal(dockerNodeReady))

		// we found the vm via plugin describe and it's in ok state
		require.NoError(t, group["case1"].Signal(instanceOK))

		// another docker node ls, and it's still in ready
		require.NoError(t, group["case1"].Signal(dockerNodeReady))

		// node does a swarm demote and leave -- docker node ls shows Down
		require.NoError(t, group["case1"].Signal(dockerNodeDown))

		// Down is unavailable
		require.Equal(t, swarmNodeDown, group["case1"].State())

		// the vm is deleted -- diff of successive 'instance describe' calls show it's gone
		require.NoError(t, group["case1"].Signal(instanceGone))

		// instance is gone but node is there
		require.Equal(t, matchedDockerNode, group["case1"].State())

		// wait a bit longer and no news from future instance describes
		time.Sleep(time.Duration(waitDescribeInstances+1) * pollInterval)

		// should be deleted
		require.Equal(t, removedInstance, group["case1"].State())

		// we should see the node getting removed.
		gone := <-dockerNodeRm.ch

		require.Equal(t, removedInstance, gone.State())
		require.Equal(t, group["case1"].ID(), gone.ID())

		// wait a bit and it will advance to done -- then we can clean up
		time.Sleep(time.Duration(waitBeforeCleanup+1) * pollInterval)

		require.Equal(t, done, group["case1"].State())

		// periodically clean up the deleted instances
		set.Delete(group["case1"])
		delete(group, "case1")
		require.Equal(t, 0, set.CountByState(done))
		require.Equal(t, 0, set.Size())
	}

	{
		// case2 - node fails to join
		group["case2"] = set.Add(start)

		// we found the vm via plugin describe and it's in ok state
		require.NoError(t, group["case2"].Signal(instanceOK))

		// there's a limit on dockerNodeJoin timeout
		time.Sleep(time.Duration(dockerNodeJoin+1) * pollInterval)

		// waiting to be deleted... unless the swarm node shows up in time!
		require.Equal(t, pendingInstanceDestroy, group["case2"].State())

		// we should see the node getting removed.
		gone := <-instanceDestroy.ch

		require.Equal(t, removedInstance, gone.State())
		require.Equal(t, group["case2"].ID(), gone.ID())

		// wait a bit and it will advance to done -- then we can clean up
		time.Sleep(time.Duration(waitBeforeCleanup+1) * pollInterval)

		require.Equal(t, done, group["case2"].State())

		// periodically clean up the deleted instances
		set.Delete(group["case2"])
		delete(group, "case2")
		require.Equal(t, 0, set.CountByState(removedInstance))
		require.Equal(t, 0, set.Size())
	}

	{
		// case3 - node / engine goes offline
		// in this case we have to do both docker node rm *and* instance destroy

		group["case3"] = set.Add(start)

		// we found the vm via plugin describe and it's in ok state
		require.NoError(t, group["case3"].Signal(instanceOK))

		// we found the engine status via docker node ls and it's in Ready state
		require.NoError(t, group["case3"].Signal(dockerNodeReady))

		// we are now matched
		require.Equal(t, swarmNode, group["case3"].State())

		// docker node ls gives ready again
		require.NoError(t, group["case3"].Signal(dockerNodeReady))

		time.Sleep(5 * pollInterval) // after a while

		// a working ready swarm node
		require.Equal(t, swarmNodeReady, group["case3"].State())

		// the node goes offline -- engine disappeared / network partition
		require.NoError(t, group["case3"].Signal(dockerNodeDown))

		// now we are in a down state
		require.Equal(t, swarmNodeDown, group["case3"].State())

		// we still see the matching instance
		require.NoError(t, group["case3"].Signal(instanceOK))

		// we are still in the Down state
		require.Equal(t, swarmNodeDown, group["case3"].State())

		// after some wait
		time.Sleep(time.Duration(waitBeforeReprovision+1) * pollInterval)

		// the instance should be ready to be destroyed
		require.Equal(t, pendingInstanceDestroy, group["case3"].State())

		// after a while, the engine still doesn't come back on within the limit...
		time.Sleep(time.Duration(waitBeforeInstanceDestroy+1) * pollInterval)

		// we should see the instance getting removed.
		gone := <-instanceDestroy.ch

		require.Equal(t, removedInstance, gone.State())
		require.Equal(t, group["case3"].ID(), gone.ID())

		// at this point, the instance is gone... but we still have a docker node ls entry in Down state
		// so another docker node ls shows this

		require.NoError(t, group["case3"].Signal(dockerNodeDown))

		// we should see the node getting removed.
		gone2 := <-dockerNodeRm.ch

		// final state
		require.Equal(t, done, gone2.State())

		// clean up
		set.Delete(group["case3"])
		delete(group, "case3")
		require.Equal(t, 0, set.CountByState(done))
		require.Equal(t, 0, set.Size())

	}

	{
		// case4 - rouge node -- added outside of infrakit's control
		// this is when a docker node shows up and we have no idea where that's from

		group["case4"] = set.Add(start)

		// we found the engine status via docker node ls and it's in Ready state
		require.NoError(t, group["case4"].Signal(dockerNodeReady))

		// now wait for an instance to show up from instance describe
		require.Equal(t, matchedDockerNode, group["case4"].State())

		// after some wait
		time.Sleep(time.Duration(waitDescribeInstances+1) * pollInterval)

		require.Equal(t, removedInstance, group["case4"].State())

		// we should see the node get removed
		gone := <-dockerNodeRm.ch

		require.Equal(t, removedInstance, gone.State())
		require.Equal(t, group["case4"].ID(), gone.ID())

		// now the node is gone, but we don't have any more instance information because
		// this instance wasn't adding by infrakit
		time.Sleep(time.Duration(waitBeforeCleanup+1) * pollInterval)

		// final state
		require.Equal(t, done, gone.State())

		// clean up
		set.Delete(group["case4"])
		delete(group, "case4")
		require.Equal(t, 0, set.CountByState(done))
		require.Equal(t, 0, set.Size())

	}

}
