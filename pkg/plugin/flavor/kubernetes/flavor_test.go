package kubernetes

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
)

func templ(tpl string) *template.Template {
	t, err := template.NewTemplate("str://"+tpl, template.Options{})
	if err != nil {
		panic(err)
	}
	return t
}

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

	managerFlavor := NewManagerFlavor(plugins, templ(DefaultManagerInitScriptTemplate), curdir, managerStop)
	workerFlavor := NewWorkerFlavor(plugins, templ(DefaultWorkerInitScriptTemplate), curdir, workerStop)
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
		group_types.AllocationMethod{LogicalIDs: []instance.LogicalID{"127.0.0.1"}}))
	require.NoError(t, workerFlavor.Validate(
		types.AnyString(`{"KubeJoinIP" : "127.0.0.1", 
		"KubeBindPort" : 6443, 
		"KubeClusterID" : "test"}`),
		group_types.AllocationMethod{Size: 5}))

	err = managerFlavor.Validate(
		types.AnyString(`{"KubeJoinIP" : "127.0.0.1", 
		"KubeBindPort" : 6443, 
		"KubeAddOns" : [ 
		    {
				"Name" : "flannel", 
				"Type" : "network",
				"Path" : "http://docs.projectcalico.org/v2.2/getting-started/kubernetes/installation/hosted/kubeadm/1.6/calico.yaml"}],
		"KubeClusterID" : "test"}`),
		group_types.AllocationMethod{LogicalIDs: []instance.LogicalID{"127.0.0.1", "127.0.0.2"}})
	require.Error(t, err)
	require.Equal(t, "kubernetes flaver currently support only one manager", err.Error())
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
	managerFlavor := NewManagerFlavor(plugins, templ(DefaultManagerInitScriptTemplate), curdir, managerStop)
	index := group_types.Index{Group: group.ID("group"), Sequence: 0}
	id := instance.LogicalID("10.20.100.1")
	flavorSpec := types.AnyString(`{"KubeJoinIP" : "10.20.100.1", 
		"KubeBindPort" : 6443, 
		"KubeAddOns" : [ 
			{
				"Name" : "flannel", 
				"Type" : "network",
				"Path" : "http://docs.projectcalico.org/v2.2/getting-started/kubernetes/installation/hosted/kubeadm/1.6/calico.yaml"}],
		"KubeClusterID" : "test",
		"Attachments": {"10.20.100.1": [{"ID": "a", "Type": "ebs"}, {"ID": "b", "Type": "ebs"}]}}`)
	details, err := managerFlavor.Prepare(flavorSpec,
		instance.Spec{Tags: map[string]string{"a": "b"}, LogicalID: &id},
		group_types.AllocationMethod{LogicalIDs: []instance.LogicalID{"10.20.100.1"}},
		index)
	require.NoError(t, err)
	_, err = os.Stat(cfdir)
	require.NoError(t, err)
	cf := path.Join(cfdir, "kubeadm-token")
	_, err = os.Stat(cfdir)
	require.NoError(t, err)
	d, err := ioutil.ReadFile(cf)
	err = os.RemoveAll(cfdir)
	require.NoError(t, err)
	i := strings.Index(details.Init, "kubeadm init --token "+string(d))
	require.NotEqual(t, -1, i)
	close(managerStop)
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
	workerFlavor := NewWorkerFlavor(plugins, templ(DefaultWorkerInitScriptTemplate), curdir, workerStop)
	index := group_types.Index{Group: group.ID("group"), Sequence: 0}
	flavorSpec := types.AnyString(`{"KubeJoinIP" : "10.20.100.1",
		"KubeBindPort" : 6443,
		"KubeClusterID" : "test",
		"SkipManagerValidation" : true,
		"Attachments": {"10.20.100.1": [{"ID": "a", "Type": "ebs"}, {"ID": "b", "Type": "ebs"}]}}`)
	_, err = workerFlavor.Prepare(flavorSpec,
		instance.Spec{Tags: map[string]string{"a": "b"}},
		group_types.AllocationMethod{Size: 5},
		index)
	require.Error(t, err)
	err = os.Mkdir(cfdir, 0777)
	require.NoError(t, err)
	cf := path.Join(cfdir, "kubeadm-token")
	testtoken := "26976b.2f0e7389eeaab3c8"
	ioutil.WriteFile(cf, []byte(testtoken), 0666)
	details, err := workerFlavor.Prepare(flavorSpec,
		instance.Spec{Tags: map[string]string{"a": "b"}},
		group_types.AllocationMethod{Size: 5},
		index)
	require.NoError(t, err)
	err = os.RemoveAll(cfdir)
	require.Equal(t, "b", details.Tags["a"])
	i := strings.Index(details.Init, "kubeadm join --token "+testtoken+" 10.20.100.1:6443")
	require.NotEqual(t, -1, i)
}
