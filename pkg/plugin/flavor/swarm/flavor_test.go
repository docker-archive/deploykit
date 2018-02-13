package swarm

import (
	"fmt"
	"testing"

	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	mock_client "github.com/docker/infrakit/pkg/mock/docker/docker/client"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var scp = scope.DefaultScope(func() discovery.Plugins {
	d, err := local.NewPluginDiscovery()
	if err != nil {
		panic(err)
	}
	return d
})

func templ(tpl string) *template.Template {
	t, err := template.NewTemplate("str://"+tpl, template.Options{})
	if err != nil {
		panic(err)
	}
	return t
}

func TestValidate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	managerStop := make(chan struct{})
	workerStop := make(chan struct{})

	managerFlavor := NewManagerFlavor(scp, func(Spec) (docker.APIClientCloser, error) {
		return mock_client.NewMockAPIClientCloser(ctrl), nil
	}, templ(DefaultManagerInitScriptTemplate), managerStop)
	workerFlavor := NewWorkerFlavor(scp, func(Spec) (docker.APIClientCloser, error) {
		return mock_client.NewMockAPIClientCloser(ctrl), nil
	}, templ(DefaultWorkerInitScriptTemplate), workerStop)

	require.NoError(t, workerFlavor.Validate(
		types.AnyString(`{"Docker" : {"Host":"unix:///var/run/docker.sock"}}`),
		group.AllocationMethod{Size: 5}))
	require.NoError(t, managerFlavor.Validate(
		types.AnyString(`{"Docker" : {"Host":"unix:///var/run/docker.sock"}}`),
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"127.0.0.1"}}))

	// Logical ID with multiple attachments is allowed.
	require.NoError(t, managerFlavor.Validate(
		types.AnyString(`{
                        "Docker" : {"Host":"unix:///var/run/docker.sock"},
			"Attachments": {"127.0.0.1": [{"ID": "a", "Type": "ebs"}, {"ID": "b", "Type": "ebs"}]}}`),
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"127.0.0.1"}}))

	// Logical ID used more than once.
	err := managerFlavor.Validate(
		types.AnyString(`{"Docker":{"Host":"unix:///var/run/docker.sock"}}`),
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"127.0.0.1", "127.0.0.1", "127.0.0.2"}})
	require.Error(t, err)
	require.Equal(t, "LogicalID 127.0.0.1 specified more than once", err.Error())

	// Attachment cannot be associated with multiple Logical IDs.
	err = managerFlavor.Validate(
		types.AnyString(`{
                        "Docker" : {"Host":"unix:///var/run/docker.sock"},
			"Attachments": {"127.0.0.1": [{"ID": "a", "Type": "ebs"}], "127.0.0.2": [{"ID": "a", "Type": "ebs"}]}}`),
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"127.0.0.1", "127.0.0.2", "127.0.0.3"}})
	require.Error(t, err)
	require.Equal(t, "Attachment a specified more than once", err.Error())

	// Attachment for all
	err = managerFlavor.Validate(
		types.AnyString(`{
                        "Docker" : {"Host":"unix:///var/run/docker.sock"},
			"Attachments": {"*": [{"ID": "a", "Type": "NFSVolume"}]}}`),
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"127.0.0.1"}})
	require.NoError(t, err)

	close(managerStop)
	close(workerStop)
}

func TestWorker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	workerStop := make(chan struct{})

	client := mock_client.NewMockAPIClientCloser(ctrl)

	flavorImpl := NewWorkerFlavor(scp, func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, templ(DefaultWorkerInitScriptTemplate), workerStop)

	swarmInfo := swarm.Swarm{
		ClusterInfo: swarm.ClusterInfo{ID: "ClusterUUID"},
		JoinTokens: swarm.JoinTokens{
			Manager: "ManagerToken",
			Worker:  "WorkerToken",
		},
	}

	client.EXPECT().SwarmInspect(gomock.Any()).Return(swarmInfo, nil).AnyTimes()
	client.EXPECT().Info(gomock.Any()).Return(infoResponse, nil).AnyTimes()
	nodeInfo := swarm.Node{ManagerStatus: &swarm.ManagerStatus{Addr: "1.2.3.4"}}
	client.EXPECT().NodeInspectWithRaw(gomock.Any(), nodeID).Return(nodeInfo, nil, nil).AnyTimes()
	client.EXPECT().Close().AnyTimes()

	index := group.Index{Group: group.ID("group"), Sequence: 0}
	details, err := flavorImpl.Prepare(
		types.AnyString(`{}`),
		instance.Spec{Tags: map[string]string{"a": "b"}},
		group.AllocationMethod{Size: 5},
		index)
	require.NoError(t, err)
	require.Equal(t, "b", details.Tags["a"])

	link := types.NewLinkFromMap(details.Tags)
	require.True(t, link.Valid())
	require.True(t, len(link.KVPairs()) > 0)

	// Perform a rudimentary check to ensure that the expected fields are in the InitScript, without having any
	// other knowledge about the script structure.
	associationID := link.Value()
	associationTag := link.Label()
	require.Contains(t, details.Init, associationID)
	require.Contains(t, details.Init, swarmInfo.JoinTokens.Worker)
	require.NotContains(t, details.Init, swarmInfo.JoinTokens.Manager)

	require.Empty(t, details.Attachments)

	// An instance with no association information is considered unhealthy.
	health, err := flavorImpl.Healthy(types.AnyString("{}"), instance.Description{})
	require.NoError(t, err)
	require.Equal(t, flavor.Unhealthy, health)

	// Worker with no state defined
	filter, err := filters.FromParam(fmt.Sprintf(`{"label": {"%s=%s": true}}`, associationTag, associationID))
	require.NoError(t, err)
	client.EXPECT().NodeList(gomock.Any(), docker_types.NodeListOptions{Filters: filter}).Return(
		[]swarm.Node{
			{},
		}, nil)
	health, err = flavorImpl.Healthy(
		types.AnyString("{}"),
		instance.Description{Tags: map[string]string{associationTag: associationID}})
	require.NoError(t, err)
	require.Equal(t, flavor.Unhealthy, health)

	// Worker that is down
	client.EXPECT().NodeList(gomock.Any(), docker_types.NodeListOptions{Filters: filter}).Return(
		[]swarm.Node{
			{
				Status: swarm.NodeStatus{
					State: swarm.NodeStateDown,
				},
			},
		}, nil)
	health, err = flavorImpl.Healthy(
		types.AnyString("{}"),
		instance.Description{Tags: map[string]string{associationTag: associationID}})
	require.NoError(t, err)
	require.Equal(t, flavor.Unhealthy, health)

	// Worker that is ready
	client.EXPECT().NodeList(gomock.Any(), docker_types.NodeListOptions{Filters: filter}).Return(
		[]swarm.Node{
			{
				Status: swarm.NodeStatus{
					State: swarm.NodeStateReady,
				},
			},
		}, nil)
	health, err = flavorImpl.Healthy(
		types.AnyString("{}"),
		instance.Description{Tags: map[string]string{associationTag: associationID}})
	require.NoError(t, err)
	require.Equal(t, flavor.Healthy, health)

	close(workerStop)
}

const nodeID = "my-node-id"

var infoResponse = docker_types.Info{Swarm: swarm.Info{NodeID: nodeID}}

func TestManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	selfAddr := "1.2.3.4"
	managerStop := make(chan struct{})

	client := mock_client.NewMockAPIClientCloser(ctrl)

	flavorImpl := NewManagerFlavor(scp, func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, templ(DefaultManagerInitScriptTemplate), managerStop)

	swarmInfo := swarm.Swarm{
		ClusterInfo: swarm.ClusterInfo{ID: "ClusterUUID"},
		JoinTokens: swarm.JoinTokens{
			Manager: "ManagerToken",
			Worker:  "WorkerToken",
		},
	}

	client.EXPECT().SwarmInspect(gomock.Any()).Return(swarmInfo, nil).AnyTimes()
	client.EXPECT().Info(gomock.Any()).Return(infoResponse, nil).AnyTimes()
	nodeInfo := swarm.Node{ManagerStatus: &swarm.ManagerStatus{Addr: selfAddr}}
	client.EXPECT().NodeInspectWithRaw(gomock.Any(), nodeID).Return(nodeInfo, nil, nil).AnyTimes()
	client.EXPECT().Close().AnyTimes()

	flavorSpec := types.AnyString(`
{
  "Attachments" : {
    "10.20.100.1" : [ { "ID" : "disk01", "Type" : "disk" }, { "ID" : "nic01", "Type" : "nic" } ],
    "10.20.100.2" : [ { "ID" : "disk02", "Type" : "disk" }, { "ID" : "nic02", "Type" : "nic" } ],
    "10.20.100.3" : [ { "ID" : "disk03", "Type" : "disk" }, { "ID" : "nic03", "Type" : "nic" } ]
  }
}
`)

	index := group.Index{Group: group.ID("group"), Sequence: 0}
	id := instance.LogicalID("10.20.100.1")
	details, err := flavorImpl.Prepare(flavorSpec,
		instance.Spec{Tags: map[string]string{"a": "b"}, LogicalID: &id},
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"10.20.100.1"}},
		index)
	require.NoError(t, err)
	require.Equal(t, "b", details.Tags["a"])

	// ensures that the attachments are matched to the logical ID of the instance and in the attachments map
	require.Equal(t, []instance.Attachment{{ID: "disk01", Type: "disk"}, {ID: "nic01", Type: "nic"}}, details.Attachments)

	link := types.NewLinkFromMap(details.Tags)
	require.True(t, link.Valid())
	require.True(t, len(link.KVPairs()) > 0)

	// Perform a rudimentary check to ensure that the expected fields are in the InitScript, without having any
	// other knowledge about the script structure.

	associationID := link.Value()
	associationTag := link.Label()
	require.Contains(t, details.Init, associationID)

	// another instance -- note that this id is not the first in the allocation list of logical ids.
	index = group.Index{Group: group.ID("group"), Sequence: 1}
	id = instance.LogicalID("172.200.100.2")
	details, err = flavorImpl.Prepare(
		types.AnyString(`{"Attachments": {"172.200.100.2": [{"ID": "a", "Type": "gpu"}]}}`),
		instance.Spec{Tags: map[string]string{"a": "b"}, LogicalID: &id},
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"172.200.100.1", "172.200.100.2"}},
		index)
	require.NoError(t, err)

	require.Contains(t, details.Init, swarmInfo.JoinTokens.Manager)
	require.NotContains(t, details.Init, swarmInfo.JoinTokens.Worker)

	require.Equal(t, []instance.Attachment{{ID: "a", Type: "gpu"}}, details.Attachments)

	// Shared AllInstances for attachment
	for _, id := range []instance.LogicalID{
		instance.LogicalID("10.20.100.1"),
		instance.LogicalID("10.20.100.2"),
		instance.LogicalID("10.20.100.3"),
	} {

		details, err = flavorImpl.Prepare(
			types.AnyString(`{"Attachments": {"*": [{"ID": "nfs", "Type": "NFSVolume"}]}}`),
			instance.Spec{Tags: map[string]string{"a": "b"}, LogicalID: &id},
			group.AllocationMethod{LogicalIDs: []instance.LogicalID{"10.20.100.1", "10.20.100.2", "10.20.100.3"}},
			index)
		require.NoError(t, err)
		require.Equal(t, []instance.Attachment{{ID: "nfs", Type: "NFSVolume"}}, details.Attachments)
	}

	// Shared AllInstances for attachment -- for workers / no special logical IDs
	details, err = flavorImpl.Prepare(
		types.AnyString(`{"Attachments": {"*": [{"ID": "nfs", "Type": "NFSVolume"}]}}`),
		instance.Spec{Tags: map[string]string{"a": "b"}},
		group.AllocationMethod{Size: 10},
		index)
	require.NoError(t, err)
	require.Equal(t, []instance.Attachment{{ID: "nfs", Type: "NFSVolume"}}, details.Attachments)

	// An instance with no association information is considered unhealthy.
	health, err := flavorImpl.Healthy(types.AnyString("{}"), instance.Description{})
	require.NoError(t, err)
	require.Equal(t, flavor.Unhealthy, health)

	// Manager that does not have any status
	filter, err := filters.FromParam(fmt.Sprintf(`{"label": {"%s=%s": true}}`, associationTag, associationID))
	require.NoError(t, err)
	client.EXPECT().NodeList(gomock.Any(), docker_types.NodeListOptions{Filters: filter}).Return(
		[]swarm.Node{
			{
				Spec: swarm.NodeSpec{
					Role: swarm.NodeRoleManager,
				},
				Status: swarm.NodeStatus{
					State: swarm.NodeStateReady,
				},
			},
		}, nil)
	health, err = flavorImpl.Healthy(
		types.AnyString("{}"),
		instance.Description{Tags: map[string]string{associationTag: associationID}})
	require.NoError(t, err)
	require.Equal(t, flavor.Unhealthy, health)

	// Manager that that is not reachable
	client.EXPECT().NodeList(gomock.Any(), docker_types.NodeListOptions{Filters: filter}).Return(
		[]swarm.Node{
			{
				ManagerStatus: &swarm.ManagerStatus{
					Reachability: swarm.ReachabilityUnknown,
				},
				Spec: swarm.NodeSpec{
					Role: swarm.NodeRoleManager,
				},
				Status: swarm.NodeStatus{
					State: swarm.NodeStateReady,
				},
			},
		}, nil)
	health, err = flavorImpl.Healthy(
		types.AnyString("{}"),
		instance.Description{Tags: map[string]string{associationTag: associationID}})
	require.NoError(t, err)
	require.Equal(t, flavor.Unhealthy, health)

	// Manager that is reachable
	client.EXPECT().NodeList(gomock.Any(), docker_types.NodeListOptions{Filters: filter}).Return(
		[]swarm.Node{
			{
				ManagerStatus: &swarm.ManagerStatus{
					Reachability: swarm.ReachabilityReachable,
				},
				Spec: swarm.NodeSpec{
					Role: swarm.NodeRoleManager,
				},
				Status: swarm.NodeStatus{
					State: swarm.NodeStateReady,
				},
			},
		}, nil)
	health, err = flavorImpl.Healthy(
		types.AnyString("{}"),
		instance.Description{Tags: map[string]string{associationTag: associationID}})
	require.NoError(t, err)
	require.Equal(t, flavor.Healthy, health)

	close(managerStop)
}

func TestTemplateFunctions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	selfAddr := "1.2.3.4"
	managerStop := make(chan struct{})

	client := mock_client.NewMockAPIClientCloser(ctrl)

	flavorImpl := NewManagerFlavor(scp, func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, templ(DefaultManagerInitScriptTemplate), managerStop)

	swarmInfo := swarm.Swarm{
		ClusterInfo: swarm.ClusterInfo{ID: "ClusterUUID"},
		JoinTokens: swarm.JoinTokens{
			Manager: "ManagerToken",
			Worker:  "WorkerToken",
		},
	}

	client.EXPECT().SwarmInspect(gomock.Any()).Return(swarmInfo, nil).AnyTimes()
	client.EXPECT().Info(gomock.Any()).Return(infoResponse, nil).AnyTimes()
	nodeInfo := swarm.Node{ManagerStatus: &swarm.ManagerStatus{Addr: selfAddr}}
	client.EXPECT().NodeInspectWithRaw(gomock.Any(), nodeID).Return(nodeInfo, nil, nil).AnyTimes()
	client.EXPECT().Close().AnyTimes()

	initTemplate := `{{/* totally not useful init script just for test*/}}{{ INDEX.Group }},{{ INDEX.Sequence }}`

	properties := types.AnyString(`
{
 "Attachments": {"10.20.100.1": [{"ID": "a", "Type": "gpu"}]},
 "InitScriptTemplateURL" : "str://` + initTemplate + `"
}
`)

	index := group.Index{Group: group.ID("group"), Sequence: 100}
	id := instance.LogicalID("10.20.100.1")
	details, err := flavorImpl.Prepare(properties,
		instance.Spec{Tags: map[string]string{"a": "b"}, LogicalID: &id},
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"10.20.100.1"}},
		index)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%v,%v", index.Group, index.Sequence), details.Init)

	close(managerStop)
}

func TestInitScriptMultipass(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	managerStop := make(chan struct{})

	client := mock_client.NewMockAPIClientCloser(ctrl)

	flavorImpl := NewManagerFlavor(scp, func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, templ(DefaultManagerInitScriptTemplate), managerStop)

	client.EXPECT().SwarmInspect(gomock.Any()).Return(swarm.Swarm{}, nil).AnyTimes()
	client.EXPECT().Info(gomock.Any()).Return(infoResponse, nil).AnyTimes()
	client.EXPECT().NodeInspectWithRaw(gomock.Any(), nodeID).Return(swarm.Node{}, nil, nil).AnyTimes()
	client.EXPECT().Close().AnyTimes()

	// The `InitScriptTemplateURL` vars should not resolve since multiple is enabled
	flavorSpec := types.AnyString(`
	{
	  "Attachments": {},
	  "InitScriptTemplateURL": "str://init {{ var \"some-var\" }}"
	}
	`)

	details, err := flavorImpl.Prepare(flavorSpec,
		instance.Spec{},
		group.AllocationMethod{},
		group.Index{Group: group.ID("group"), Sequence: 100})
	require.NoError(t, err)
	require.Equal(t, "init {{ var `some-var` }}", details.Init)

	close(managerStop)
}
