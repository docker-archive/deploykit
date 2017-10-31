package kubernetes

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
)

func plugins() discovery.Plugins {
	d, err := local.NewPluginDiscovery()
	if err != nil {
		panic(err)
	}
	return d
}

func TestValidate(t *testing.T) {
	managerStop := make(chan struct{})
	workerStop := make(chan struct{})
	curdir, err := os.Getwd()
	require.NoError(t, err)

	options := Options{
		ConfigDir: curdir,
	}
	managerFlavor, err := NewManagerFlavor(scope.DefaultScope(plugins), options, managerStop)
	require.NoError(t, err)

	workerFlavor, err := NewWorkerFlavor(scope.DefaultScope(plugins), options, workerStop)
	require.NoError(t, err)

	// Logical ID with multiple attachments is allowed.
	require.NoError(t, workerFlavor.Validate(
		types.AnyString(`{"KubeJoinIP" : "127.0.0.1", 
		"KubeBindPort" : 6443, 
		"KubeAddOns" : [
			{
				"Name" : "flannel", 
				"Type" : "network",
				"Path" : "http://docs.projectcalico.org/v2.2/getting-started/kubernetes/installation/hosted/kubeadm/1.6/calico.yaml"}],
		"KubeClusterID" : "test",
		"Attachments": {"127.0.0.1": [{"ID": "a", "Type": "ebs"}, {"ID": "b", "Type": "ebs"}]}}`),
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"127.0.0.1"}}))
	require.NoError(t, workerFlavor.Validate(
		types.AnyString(`{"KubeJoinIP" : "127.0.0.1", 
		"KubeBindPort" : 6443, 
		"KubeClusterID" : "test"}`),
		group.AllocationMethod{Size: 5}))

	err = managerFlavor.Validate(
		types.AnyString(`{"KubeJoinIP" : "127.0.0.1", 
		"KubeBindPort" : 6443, 
		"KubeAddOns" : [ 
		    {
				"Name" : "flannel", 
				"Type" : "network",
				"Path" : "http://docs.projectcalico.org/v2.2/getting-started/kubernetes/installation/hosted/kubeadm/1.6/calico.yaml"}],
		"KubeClusterID" : "test"}`),
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"127.0.0.1", "127.0.0.2", "127.0.0.3"}})
	require.NoError(t, err)
	close(managerStop)
	close(workerStop)
}

func TestManager(t *testing.T) {
	managerStop := make(chan struct{})
	curdir, err := os.Getwd()
	require.NoError(t, err)
	cfdir := path.Join(curdir, "infrakit-kube-test")
	_, err = os.Stat(cfdir)
	if err == nil {
		err = os.RemoveAll(cfdir)
		require.NoError(t, err)
	}

	managerFlavor, err := NewManagerFlavor(scope.DefaultScope(plugins), Options{ConfigDir: curdir}, managerStop)
	require.NoError(t, err)

	index := group.Index{Group: group.ID("group"), Sequence: 0}
	id := instance.LogicalID("10.20.100.1")
	flavorSpec := types.AnyString(`{"KubeJoinIP" : "10.20.100.1", 
		"KubeBindPort" : 6443, 
		"KubeAddOns" : [ 
			{
				"Name" : "flannel", 
				"Type" : "network",
				"Path" : "http://docs.projectcalico.org/v2.2/getting-started/kubernetes/installation/hosted/kubeadm/1.6/calico.yaml"}],
		"KubeClusterID" : "test",
                "SkipManagerValidation" : true,
                "ControlPlane" : [ "10.20.100.1" ],
		"Attachments": {"10.20.100.1": [{"ID": "a", "Type": "ebs"}, {"ID": "b", "Type": "ebs"}]}}`)
	details, err := managerFlavor.Prepare(flavorSpec,
		instance.Spec{Tags: map[string]string{"a": "b"}, LogicalID: &id},
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"10.20.100.1", "10.20.100.2", "10.20.100.3"}},
		index)
	require.NoError(t, err)
	_, err = os.Stat(cfdir)
	require.NoError(t, err)
	cf := path.Join(cfdir, "kubeadm-token")
	_, err = os.Stat(cfdir)
	require.NoError(t, err)
	d, err := ioutil.ReadFile(cf)
	require.NoError(t, err)

	i := strings.Index(details.Init, "kubeadm init --token "+string(d))
	require.NotEqual(t, -1, i)

	// Second node in manager group but it's not a control plane for kube
	id = instance.LogicalID("10.20.100.2")
	index = group.Index{Group: group.ID("group"), Sequence: 1}
	details, err = managerFlavor.Prepare(flavorSpec,
		instance.Spec{Tags: map[string]string{"a": "b"}, LogicalID: &id},
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"10.20.100.1", "10.20.100.2", "10.20.100.3"}},
		index)
	require.NoError(t, err)
	require.Equal(t, -1, strings.Index(details.Init, "kubeadm init --token "+string(d)))
	require.True(t, strings.Index(details.Init, "kubeadm join --token "+string(d)) > -1)

	// Last node in manager group but it's not a control plane for kube
	id = instance.LogicalID("10.20.100.3")
	index = group.Index{Group: group.ID("group"), Sequence: 2}
	details, err = managerFlavor.Prepare(flavorSpec,
		instance.Spec{Tags: map[string]string{"a": "b"}, LogicalID: &id},
		group.AllocationMethod{LogicalIDs: []instance.LogicalID{"10.20.100.1", "10.20.100.2", "10.20.100.3"}},
		index)
	require.NoError(t, err)
	require.Equal(t, -1, strings.Index(details.Init, "kubeadm init --token "+string(d)))
	require.True(t, strings.Index(details.Init, "kubeadm join --token "+string(d)) > -1)

	close(managerStop)

	// clean up
	require.NoError(t, os.RemoveAll(cfdir))
}

func TestSubset(t *testing.T) {

	require.True(t, strictSubset(
		[]instance.LogicalID{"10.20.100.1", "10.20.100.2", "10.20.100.3"},
		[]instance.LogicalID{"10.20.100.1", "10.20.100.2", "10.20.100.3"},
	))

	require.True(t, strictSubset(
		[]instance.LogicalID{"10.20.100.1"},
		[]instance.LogicalID{"10.20.100.1", "10.20.100.2", "10.20.100.3"},
	))

	require.False(t, strictSubset(
		[]instance.LogicalID{"10.20.100.1", "10.20.100.2", "10.20.100.3"},
		[]instance.LogicalID{"10.20.100.1"},
	))
	require.False(t, strictSubset(
		[]instance.LogicalID{"10.20.100.1", "10.20.100.2", "10.20.100.3"},
		[]instance.LogicalID{"10.20.100.1", "10.20.100.2", "10.20.100.4"},
	))
}

func TestWorker(t *testing.T) {
	workerStop := make(chan struct{})
	curdir, err := os.Getwd()
	require.NoError(t, err)
	cfdir := path.Join(curdir, "infrakit-kube-test")
	_, err = os.Stat(cfdir)
	if err == nil {
		err = os.RemoveAll(cfdir)
		require.NoError(t, err)
	}
	workerFlavor, err := NewWorkerFlavor(scope.DefaultScope(plugins), Options{ConfigDir: curdir}, workerStop)
	require.NoError(t, err)

	index := group.Index{Group: group.ID("group"), Sequence: 0}
	flavorSpec := types.AnyString(`{"KubeJoinIP" : "10.20.100.1",
		"KubeBindPort" : 6443,
		"KubeClusterID" : "test",
		"SkipManagerValidation" : true,
		"Attachments": {"10.20.100.1": [{"ID": "a", "Type": "ebs"}, {"ID": "b", "Type": "ebs"}]}}`)
	_, err = workerFlavor.Prepare(flavorSpec,
		instance.Spec{Tags: map[string]string{"a": "b"}},
		group.AllocationMethod{Size: 5},
		index)
	require.Error(t, err)
	err = os.Mkdir(cfdir, 0777)
	require.NoError(t, err)
	cf := path.Join(cfdir, "kubeadm-token")
	testtoken := "26976b.2f0e7389eeaab3c8"
	ioutil.WriteFile(cf, []byte(testtoken), 0666)
	details, err := workerFlavor.Prepare(flavorSpec,
		instance.Spec{Tags: map[string]string{"a": "b"}},
		group.AllocationMethod{Size: 5},
		index)
	require.NoError(t, err)
	err = os.RemoveAll(cfdir)
	require.Equal(t, "b", details.Tags["a"])
	i := strings.Index(details.Init, "kubeadm join --token "+testtoken+" 10.20.100.1:6443")
	require.NotEqual(t, -1, i)
}
