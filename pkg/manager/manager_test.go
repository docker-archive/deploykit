package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/leader"
	group_mock "github.com/docker/infrakit/pkg/mock/spi/group"
	store_mock "github.com/docker/infrakit/pkg/mock/store"
	"github.com/docker/infrakit/pkg/plugin"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type testLeaderDetector struct {
	t     *testing.T
	me    string
	input <-chan string
	stop  chan struct{}
	ch    chan leader.Leadership
}

func (l *testLeaderDetector) Receive() <-chan leader.Leadership {
	return l.ch
}

func (l *testLeaderDetector) Start() (<-chan leader.Leadership, error) {
	l.stop = make(chan struct{})
	l.ch = make(chan leader.Leadership)
	go func() {
		for {
			select {
			case <-l.stop:
				return
			case found := <-l.input:
				if found == l.me {
					l.ch <- leader.Leadership{Status: leader.Leader}
				} else {
					l.ch <- leader.Leadership{Status: leader.NotLeader}
				}
			}
		}
	}()
	return l.ch, nil
}

func (l *testLeaderDetector) Stop() {
	close(l.stop)
}

func testEnsemble(t *testing.T,
	dir, id string,
	leader chan string,
	ctrl *gomock.Controller,
	configStore func(*store_mock.MockSnapshot),
	configMetadataStore func(*store_mock.MockSnapshot),
	configureGroup func(*group_mock.MockPlugin)) (Backend, server.Stoppable) {

	disc, err := local.NewPluginDiscoveryWithDir(dir)
	require.NoError(t, err)

	detector := &testLeaderDetector{t: t, me: id, input: leader}

	snap := store_mock.NewMockSnapshot(ctrl)
	configStore(snap)

	metadataSnap := store_mock.NewMockSnapshot(ctrl)
	configMetadataStore(metadataSnap)

	// start group
	gm := group_mock.NewMockPlugin(ctrl)
	configureGroup(gm)

	gs := group_rpc.PluginServer(gm)
	st, err := server.StartPluginAtPath(filepath.Join(dir, "group-stateless"), gs)
	require.NoError(t, err)

	m := NewManager(scope.DefaultScope(func() discovery.Plugins { return disc }),
		Options{
			Name:      plugin.Name("group"),
			Leader:    detector,
			SpecStore: snap,
			Group:     plugin.Name("group-stateless"),
		})

	return m, st
}

func testSetLeader(t *testing.T, c []chan string, l string) {
	for _, cc := range c {
		cc <- l
	}
}

func testDiscoveryDir(t *testing.T) string {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("%v", time.Now().UnixNano()))
	err := os.MkdirAll(dir, 0744)
	require.NoError(t, err)
	return dir
}

func testBuildGroupSpec(groupID, properties string) group.Spec {
	return group.Spec{
		ID:         group.ID(groupID),
		Properties: types.AnyString(properties),
	}
}

func testBuildGlobalSpec(t *testing.T, gs group.Spec) globalSpec {
	global := globalSpec{}
	global.updateGroupSpec(gs, plugin.Name("group-stateless"))
	global.data = []entry{}
	for k, r := range global.index {
		global.data = append(global.data, entry{Key: k, Record: r})
	}
	return global
}

func testToStruct(m *types.Any) interface{} {
	o := map[string]interface{}{}
	m.Decode(&o)
	return &o
}

func testCloseAll(c []chan string) {
	for _, cc := range c {
		close(cc)
	}
}

func TestNoCallsToGroupWhenNoLeader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderChans := []chan string{make(chan string), make(chan string)}

	manager1, stoppable1 := testEnsemble(t, testDiscoveryDir(t), "m1", leaderChans[0], ctrl,
		func(s *store_mock.MockSnapshot) {
			// no calls
		},
		func(s *store_mock.MockSnapshot) {
			// no calls
		},
		func(g *group_mock.MockPlugin) {
			// no calls
		})
	manager2, stoppable2 := testEnsemble(t, testDiscoveryDir(t), "m2", leaderChans[1], ctrl,
		func(s *store_mock.MockSnapshot) {
			// no calls
		},
		func(s *store_mock.MockSnapshot) {
			// no calls
		},
		func(g *group_mock.MockPlugin) {
			// no calls
		})

	manager1.Start()
	manager2.Start()

	testSetLeader(t, leaderChans, "nobody")

	manager1.Stop()
	manager2.Stop()

	stoppable1.Stop()
	stoppable2.Stop()

	testCloseAll(leaderChans)
}

func TestStartOneLeader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gs := testBuildGroupSpec("managers", `
{
   "field1": "value1"
}
`)
	global := testBuildGlobalSpec(t, gs)

	leaderChans := []chan string{make(chan string), make(chan string)}
	checkpoint := make(chan struct{})

	lock := sync.Mutex{}
	commitCount := 0
	manager1, stoppable1 := testEnsemble(t, testDiscoveryDir(t), "m1", leaderChans[0], ctrl,
		func(s *store_mock.MockSnapshot) {
			empty := &[]entry{}
			s.EXPECT().Load(gomock.Eq(empty)).Do(
				func(o interface{}) error {
					p, is := o.(*[]entry)
					require.True(t, is)
					*p = global.data
					return nil
				}).Return(nil)
		},
		func(s *store_mock.MockSnapshot) {
			// TODO
		},
		func(g *group_mock.MockPlugin) {
			g.EXPECT().CommitGroup(gomock.Any(), false).Do(
				func(spec group.Spec, pretend bool) (string, error) {
					// Close the checkpoint channel after the 2nd commit
					lock.Lock()
					defer lock.Unlock()
					commitCount++
					if commitCount == 2 {
						defer close(checkpoint)
					}

					require.Equal(t, gs.ID, spec.ID)
					require.Equal(t, testToStruct(gs.Properties), testToStruct(spec.Properties))
					return "ok", nil
				}).Return("ok", nil).Times(2)
		})
	manager2, stoppable2 := testEnsemble(t, testDiscoveryDir(t), "m2", leaderChans[1], ctrl,
		func(s *store_mock.MockSnapshot) {
			// no calls expected
		},
		func(s *store_mock.MockSnapshot) {
			// no calls expected
		},
		func(g *group_mock.MockPlugin) {
			// no calls expected
		})

	manager1.Start()
	manager2.Start()

	testSetLeader(t, leaderChans, "m1")

	<-checkpoint

	manager1.Stop()
	manager2.Stop()

	stoppable1.Stop()
	stoppable2.Stop()

	testCloseAll(leaderChans)
}

func TestChangeLeadership(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gs := testBuildGroupSpec("managers", `
{
   "field1": "value1"
}
`)
	global := testBuildGlobalSpec(t, gs)

	leaderChans := []chan string{make(chan string), make(chan string)}
	checkpoint1 := make(chan struct{})
	checkpoint2 := make(chan struct{})
	checkpoint3 := make(chan struct{})
	lock := sync.Mutex{}

	commitCountMgr1 := 0
	manager1, stoppable1 := testEnsemble(t, testDiscoveryDir(t), "m1", leaderChans[0], ctrl,
		func(s *store_mock.MockSnapshot) {
			empty := &[]entry{}
			s.EXPECT().Load(gomock.Eq(empty)).Do(
				func(o interface{}) error {
					p, is := o.(*[]entry)
					require.True(t, is)
					*p = global.data
					return nil
				},
			).Return(nil)
		},
		func(s *store_mock.MockSnapshot) {
			// no calls expected
		},
		func(g *group_mock.MockPlugin) {
			g.EXPECT().CommitGroup(gomock.Any(), false).Do(
				func(spec group.Spec, pretend bool) (string, error) {
					// Close the checkpoint1 channel after the 2nd commit
					lock.Lock()
					defer lock.Unlock()
					commitCountMgr1++
					if commitCountMgr1 == 2 {
						defer close(checkpoint1)
					}

					require.Equal(t, gs.ID, spec.ID)
					require.Equal(t, testToStruct(gs.Properties), testToStruct(spec.Properties))
					return "ok", nil
				},
			).Return("ok", nil).Times(2)

			// We will get a call to inspect what's being watched
			g.EXPECT().InspectGroups().Return([]group.Spec{gs}, nil)

			// Now we lost leadership... need to unwatch
			g.EXPECT().FreeGroup(gomock.Eq(group.ID("managers"))).Do(
				func(id group.ID) error {

					defer close(checkpoint3)

					return nil
				},
			).Return(nil)
		})

	commitCountMgr2 := 0
	manager2, stoppable2 := testEnsemble(t, testDiscoveryDir(t), "m2", leaderChans[1], ctrl,
		func(s *store_mock.MockSnapshot) {
			empty := &[]entry{}
			s.EXPECT().Load(gomock.Eq(empty)).Do(
				func(o interface{}) error {
					p, is := o.(*[]entry)
					require.True(t, is)
					*p = global.data
					return nil
				},
			).Return(nil)
		},
		func(s *store_mock.MockSnapshot) {
			// no calls expected
		},
		func(g *group_mock.MockPlugin) {
			g.EXPECT().CommitGroup(gomock.Any(), false).Do(
				func(spec group.Spec, pretend bool) (string, error) {
					// Close the checkpoint2 channel after the 2nd commit
					lock.Lock()
					defer lock.Unlock()
					commitCountMgr2++
					if commitCountMgr2 == 2 {
						defer close(checkpoint2)
					}

					require.Equal(t, gs.ID, spec.ID)
					require.Equal(t, testToStruct(gs.Properties), testToStruct(spec.Properties))
					return "ok", nil
				},
			).Return("ok", nil).Times(2)
		})

	manager1.Start()
	manager2.Start()

	testSetLeader(t, leaderChans, "m1")

	<-checkpoint1

	testSetLeader(t, leaderChans, "m2")

	<-checkpoint2
	<-checkpoint3

	manager1.Stop()
	manager2.Stop()

	stoppable1.Stop()
	stoppable2.Stop()

	testCloseAll(leaderChans)
}
