package manager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	leader_file "github.com/docker/infrakit/pkg/leader/file"
	group_mock "github.com/docker/infrakit/pkg/mock/spi/group"
	store_mock "github.com/docker/infrakit/pkg/mock/store"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/group"
	_ "github.com/docker/infrakit/pkg/store"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func testEnsemble(t *testing.T,
	dir, id string,
	leaderFile *os.File,
	ctrl *gomock.Controller,
	configStore func(*store_mock.MockSnapshot),
	configureGroup func(*group_mock.MockPlugin)) (Backend, server.Stoppable) {

	disc, err := discovery.NewPluginDiscoveryWithDirectory(dir)
	require.NoError(t, err)

	detector, err := leader_file.NewDetector(10*time.Millisecond, leaderFile.Name(), id)
	require.NoError(t, err)

	snap := store_mock.NewMockSnapshot(ctrl)
	configStore(snap)

	// start group
	gm := group_mock.NewMockPlugin(ctrl)
	configureGroup(gm)

	gs := group_rpc.PluginServer(gm)
	st, err := server.StartPluginAtPath(filepath.Join(dir, "group-stateless"), gs)
	require.NoError(t, err)

	m, err := NewManager(disc, detector, snap, "group-stateless")
	require.NoError(t, err)

	return m, st
}

func testSetLeader(t *testing.T, f *os.File, l string) {
	err := ioutil.WriteFile(f.Name(), []byte(l), 0644)
	require.NoError(t, err)
}

func testDiscoveryDir(t *testing.T) string {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("%v", time.Now().UnixNano()))
	err := os.MkdirAll(dir, 0744)
	require.NoError(t, err)
	return dir
}

func testBuildGroupSpec(groupID, properties string) group.Spec {
	raw := json.RawMessage([]byte(properties))
	return group.Spec{
		ID:         group.ID(groupID),
		Properties: &raw,
	}
}

func testBuildGlobalSpec(t *testing.T, gs group.Spec) GlobalSpec {
	buff, err := json.Marshal(gs)
	require.NoError(t, err)
	raw := json.RawMessage(buff)
	return GlobalSpec{
		Groups: map[group.ID]PluginSpec{
			gs.ID: {
				Plugin:     "group-stateless",
				Properties: &raw,
			},
		},
	}
}

func testToStruct(m *json.RawMessage) interface{} {
	o := map[string]interface{}{}
	json.Unmarshal([]byte(*m), &o)
	return &o
}

func TestNoCallsToGroupWhenNoLeader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderFile, err := ioutil.TempFile(os.TempDir(), "infrakit-leader")
	require.NoError(t, err)

	testSetLeader(t, leaderFile, "nobody")

	manager1, stoppable1 := testEnsemble(t, testDiscoveryDir(t), "m1", leaderFile, ctrl,
		func(s *store_mock.MockSnapshot) {
			// no calls
		},
		func(g *group_mock.MockPlugin) {
			// no calls
		})
	manager2, stoppable2 := testEnsemble(t, testDiscoveryDir(t), "m2", leaderFile, ctrl,
		func(s *store_mock.MockSnapshot) {
			// no calls
		},
		func(g *group_mock.MockPlugin) {
			// no calls
		})

	manager1.Start()
	manager2.Start()

	time.Sleep(1 * time.Second)

	manager1.Stop()
	manager2.Stop()

	stoppable1.Stop()
	stoppable2.Stop()
}

func TestStartOneLeader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderFile, err := ioutil.TempFile(os.TempDir(), "infrakit-leader")
	require.NoError(t, err)

	testSetLeader(t, leaderFile, "m1")

	gs := testBuildGroupSpec("managers", `
{
   "field1": "value1"
}
`)
	global := testBuildGlobalSpec(t, gs)

	manager1, stoppable1 := testEnsemble(t, testDiscoveryDir(t), "m1", leaderFile, ctrl,
		func(s *store_mock.MockSnapshot) {
			empty := &GlobalSpec{}
			s.EXPECT().Load(gomock.Eq(empty)).Do(
				func(o interface{}) error {
					p, is := o.(*GlobalSpec)
					require.True(t, is)
					*p = global
					return nil
				}).Return(nil)
		},
		func(g *group_mock.MockPlugin) {
			g.EXPECT().CommitGroup(gomock.Any(), false).Do(
				func(spec group.Spec, pretend bool) (string, error) {
					require.Equal(t, gs.ID, spec.ID)
					require.Equal(t, testToStruct(gs.Properties), testToStruct(spec.Properties))
					return "ok", nil
				}).Return("ok", nil)
		})
	manager2, stoppable2 := testEnsemble(t, testDiscoveryDir(t), "m2", leaderFile, ctrl,
		func(s *store_mock.MockSnapshot) {
			// no calls expected
		},
		func(g *group_mock.MockPlugin) {
			// no calls expected
		})

	manager1.Start()
	manager2.Start()

	time.Sleep(1 * time.Second)

	manager1.Stop()
	manager2.Stop()

	stoppable1.Stop()
	stoppable2.Stop()

}

func TestChangeLeadership(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderFile, err := ioutil.TempFile(os.TempDir(), "infrakit-leader")
	require.NoError(t, err)

	testSetLeader(t, leaderFile, "nobody")

	gs := testBuildGroupSpec("managers", `
{
   "field1": "value1"
}
`)
	global := testBuildGlobalSpec(t, gs)

	manager1, stoppable1 := testEnsemble(t, testDiscoveryDir(t), "m1", leaderFile, ctrl,
		func(s *store_mock.MockSnapshot) {
			empty := &GlobalSpec{}
			s.EXPECT().Load(gomock.Eq(empty)).Do(
				func(o interface{}) error {
					p, is := o.(*GlobalSpec)
					require.True(t, is)
					*p = global
					return nil
				}).Return(nil)
		},
		func(g *group_mock.MockPlugin) {
			g.EXPECT().CommitGroup(gomock.Any(), false).Do(
				func(spec group.Spec, pretend bool) (string, error) {
					require.Equal(t, gs.ID, spec.ID)
					require.Equal(t, testToStruct(gs.Properties), testToStruct(spec.Properties))
					return "ok", nil
				}).Return("ok", nil)

			// We will get a call to inspect what's being watched
			g.EXPECT().InspectGroups().Return([]group.Spec{gs}, nil)

			// Now we lost leadership... need to unwatch
			g.EXPECT().FreeGroup(gomock.Eq(group.ID("managers"))).Return(nil)

		})
	manager2, stoppable2 := testEnsemble(t, testDiscoveryDir(t), "m2", leaderFile, ctrl,
		func(s *store_mock.MockSnapshot) {
			empty := &GlobalSpec{}
			s.EXPECT().Load(gomock.Eq(empty)).Do(
				func(o interface{}) error {
					p, is := o.(*GlobalSpec)
					require.True(t, is)
					*p = global
					return nil
				}).Return(nil)
		},
		func(g *group_mock.MockPlugin) {
			g.EXPECT().CommitGroup(gomock.Any(), false).Do(
				func(spec group.Spec, pretend bool) (string, error) {
					require.Equal(t, gs.ID, spec.ID)
					require.Equal(t, testToStruct(gs.Properties), testToStruct(spec.Properties))
					return "ok", nil
				}).Return("ok", nil)
		})

	manager1.Start()
	manager2.Start()

	testSetLeader(t, leaderFile, "m1")

	time.Sleep(1 * time.Second)

	testSetLeader(t, leaderFile, "m2")

	time.Sleep(1 * time.Second)

	manager1.Stop()
	manager2.Stop()

	stoppable1.Stop()
	stoppable2.Stop()

}
