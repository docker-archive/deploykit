package fsm

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	docker_node_ready Signal = iota
	docker_node_down
	docker_node_gone
	instance_ok
	instance_gone
	timeout
	reap
)

const (
	start                    Index = iota
	matched_instance               // has vm information, waiting to match to docker_node
	matched_docker_node            // has docker_node information, waiting to match to vm
	swarm_node                     // has matching docker_node and vm information
	swarm_node_ready               // ready as swarm node
	swarm_node_down                // unavailable as swarm node
	pending_instance_destroy       // vm needs to be removed_instance (instance destroy)
	removed_instance               // instance is deleted
	done                           // terminal
)

type gc struct {
	ch chan Instance
	op string
}

func (gc gc) do(i Instance) error {
	fmt.Println(gc.op, i)
	gc.ch <- i
	return nil
}

func TestSwarmEntities(t *testing.T) {

	pollInterval := 100 * time.Millisecond

	no_data := Tick(10)
	docker_node_join := Tick(5)
	wait_describe_instances := Tick(5)
	wait_before_instance_destroy := Tick(3)
	wait_before_reprovision := Tick(10) // wait before we reprovision a new instance to fix a Down node
	wait_before_cleanup := Tick(10)

	// actions
	dockerNodeRm := &gc{
		ch: make(chan Instance, 1),
		op: "dockerNodeRm",
	}

	instanceDestroy := &gc{
		ch: make(chan Instance, 1),
		op: "instanceDestroy",
	}

	model, err := Define(
		State{
			Index: start,
			TTL:   Expiry{no_data, timeout},
			Transitions: map[Signal]Index{
				docker_node_ready: matched_docker_node,
				docker_node_down:  matched_docker_node,
				instance_ok:       matched_instance,
				timeout:           removed_instance, // nothing happened... cleanup
			},
		},
		State{
			Index: matched_instance,
			TTL:   Expiry{docker_node_join, docker_node_gone},
			Transitions: map[Signal]Index{
				docker_node_ready: swarm_node,
				docker_node_down:  swarm_node,
				docker_node_gone:  pending_instance_destroy,
				instance_gone:     removed_instance,
			},
			Actions: map[Signal]Action{
				instance_gone: instanceDestroy.do,
			},
		},
		State{
			Index: pending_instance_destroy,
			TTL:   Expiry{wait_before_instance_destroy, reap},
			Transitions: map[Signal]Index{
				docker_node_ready: swarm_node, // late joiner
				docker_node_down:  swarm_node,
				instance_gone:     removed_instance,
				reap:              removed_instance,
			},
			Actions: map[Signal]Action{
				instance_gone: instanceDestroy.do,
				reap:          instanceDestroy.do,
			},
		},
		State{
			Index: matched_docker_node,
			TTL:   Expiry{wait_describe_instances, instance_gone},
			Transitions: map[Signal]Index{
				instance_ok:      swarm_node,
				instance_gone:    removed_instance,
				docker_node_gone: removed_instance, // could be docker rm'd out of band
			},
			Actions: map[Signal]Action{
				instance_gone: dockerNodeRm.do,
			},
		},
		State{
			Index: swarm_node,
			Transitions: map[Signal]Index{
				docker_node_ready: swarm_node_ready,
				docker_node_down:  swarm_node_down,
				docker_node_gone:  matched_instance,
				instance_gone:     matched_docker_node,
			},
		},
		State{
			Index: swarm_node_ready,
			Transitions: map[Signal]Index{
				docker_node_down: swarm_node_down,
				docker_node_gone: matched_instance,
				instance_gone:    matched_docker_node,
			},
		},
		State{
			Index: swarm_node_down,
			TTL:   Expiry{wait_before_reprovision, docker_node_gone},
			Transitions: map[Signal]Index{
				docker_node_ready: swarm_node_ready,
				docker_node_gone:  pending_instance_destroy,
				instance_gone:     matched_docker_node,
			},
		},
		State{
			Index: removed_instance, // after we removed the instance, we can still have unmatched node
			TTL:   Expiry{wait_before_cleanup, timeout},
			Transitions: map[Signal]Index{
				docker_node_down: done,
				timeout:          done,
			},
			Actions: map[Signal]Action{
				docker_node_down: dockerNodeRm.do,
			},
		},
		State{
			Index: done, // deleted state is terminal. this will be garbage collected
		},
	)
	require.NoError(t, err)

	clock := Wall(time.Tick(pollInterval))
	clock.Start()

	t.Log(model)

	set := NewSet(model, clock, DefaultOptions("test"))
	defer func() {
		set.Stop()
		clock.Stop()
	}()

	group := map[string]Instance{
		"case1": set.Add(start),
	}

	{
		// case 1 - orphaned ex-leader node, a unmatched "Down" node where instance was removed and deletion observed.
		// add new instance in start state
		// we found the node via docker node ls and it's in ready state
		require.NoError(t, group["case1"].Signal(docker_node_ready))

		// we found the vm via plugin describe and it's in ok state
		require.NoError(t, group["case1"].Signal(instance_ok))

		// another docker node ls, and it's still in ready
		require.NoError(t, group["case1"].Signal(docker_node_ready))

		// node does a swarm demote and leave -- docker node ls shows Down
		require.NoError(t, group["case1"].Signal(docker_node_down))

		// Down is unavailable
		require.Equal(t, swarm_node_down, group["case1"].State())

		// the vm is deleted -- diff of successive 'instance describe' calls show it's gone
		require.NoError(t, group["case1"].Signal(instance_gone))

		// instance is gone but node is there
		require.Equal(t, matched_docker_node, group["case1"].State())

		// wait a bit longer and no news from future instance describes
		time.Sleep(time.Duration(wait_describe_instances+1) * pollInterval)

		// should be deleted
		require.Equal(t, removed_instance, group["case1"].State())

		// we should see the node getting removed.
		gone := <-dockerNodeRm.ch

		require.Equal(t, removed_instance, gone.State())
		require.Equal(t, group["case1"].ID(), gone.ID())

		// wait a bit and it will advance to done -- then we can clean up
		time.Sleep(time.Duration(wait_before_cleanup+1) * pollInterval)

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
		require.NoError(t, group["case2"].Signal(instance_ok))

		// there's a limit on docker_node_join timeout
		time.Sleep(time.Duration(docker_node_join+1) * pollInterval)

		// waiting to be deleted... unless the swarm node shows up in time!
		require.Equal(t, pending_instance_destroy, group["case2"].State())

		// we should see the node getting removed.
		gone := <-instanceDestroy.ch

		require.Equal(t, removed_instance, gone.State())
		require.Equal(t, group["case2"].ID(), gone.ID())

		// wait a bit and it will advance to done -- then we can clean up
		time.Sleep(time.Duration(wait_before_cleanup+1) * pollInterval)

		require.Equal(t, done, group["case2"].State())

		// periodically clean up the deleted instances
		set.Delete(group["case2"])
		delete(group, "case2")
		require.Equal(t, 0, set.CountByState(removed_instance))
		require.Equal(t, 0, set.Size())
	}

	{
		// case3 - node / engine goes offline
		// in this case we have to do both docker node rm *and* instance destroy

		group["case3"] = set.Add(start)

		// we found the vm via plugin describe and it's in ok state
		require.NoError(t, group["case3"].Signal(instance_ok))

		// we found the engine status via docker node ls and it's in Ready state
		require.NoError(t, group["case3"].Signal(docker_node_ready))

		// we are now matched
		require.Equal(t, swarm_node, group["case3"].State())

		// docker node ls gives ready again
		require.NoError(t, group["case3"].Signal(docker_node_ready))

		time.Sleep(5 * pollInterval) // after a while

		// a working ready swarm node
		require.Equal(t, swarm_node_ready, group["case3"].State())

		// the node goes offline -- engine disappeared / network partition
		require.NoError(t, group["case3"].Signal(docker_node_down))

		// now we are in a down state
		require.Equal(t, swarm_node_down, group["case3"].State())

		// we still see the matching instance
		require.NoError(t, group["case3"].Signal(instance_ok))

		// we are still in the Down state
		require.Equal(t, swarm_node_down, group["case3"].State())

		// after some wait
		time.Sleep(time.Duration(wait_before_reprovision+1) * pollInterval)

		// the instance should be ready to be destroyed
		require.Equal(t, pending_instance_destroy, group["case3"].State())

		// after a while, the engine still doesn't come back on within the limit...
		time.Sleep(time.Duration(wait_before_instance_destroy+1) * pollInterval)

		// we should see the instance getting removed.
		gone := <-instanceDestroy.ch

		require.Equal(t, removed_instance, gone.State())
		require.Equal(t, group["case3"].ID(), gone.ID())

		// at this point, the instance is gone... but we still have a docker node ls entry in Down state
		// so another docker node ls shows this

		require.NoError(t, group["case3"].Signal(docker_node_down))

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
		require.NoError(t, group["case4"].Signal(docker_node_ready))

		// now wait for an instance to show up from instance describe
		require.Equal(t, matched_docker_node, group["case4"].State())

		// after some wait
		time.Sleep(time.Duration(wait_describe_instances+1) * pollInterval)

		require.Equal(t, removed_instance, group["case4"].State())

		// we should see the node get removed
		gone := <-dockerNodeRm.ch

		require.Equal(t, removed_instance, gone.State())
		require.Equal(t, group["case4"].ID(), gone.ID())

		// now the node is gone, but we don't have any more instance information because
		// this instance wasn't adding by infrakit
		time.Sleep(time.Duration(wait_before_cleanup+1) * pollInterval)

		// final state
		require.Equal(t, done, gone.State())

		// clean up
		set.Delete(group["case4"])
		delete(group, "case4")
		require.Equal(t, 0, set.CountByState(done))
		require.Equal(t, 0, set.Size())

	}

}
