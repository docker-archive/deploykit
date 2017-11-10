package vsphere

import (
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/provider/vsphere"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "vsphere"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_VSPHERE_NAMESPACE_TAGS"

	// EnvVCAlias is an alias that is given to the plugin
	EnvVCAlias = "INFRAKIT_VSPHERE_VCALIAS"

	// EnvVCURL is the env for setting the VCenter URL to connect to
	EnvVCURL = "INFRAKIT_VSPHERE_VCURL"

	// EnvVCDataCenter is the VCenter data center name
	EnvVCDataCenter = "INFRAKIT_VSPHERE_VCDATACENTER"

	// EnvVCDataStore is the VCenter data store name
	EnvVCDataStore = "INFRAKIT_VSPHERE_VCDATASTORE"

	// EnvVCNetwork is the Network name
	EnvVCNetwork = "INFRAKIT_VSPHERE_VCNETWORK"

	// EnvVCHost is the host that will run the VM
	EnvVCHost = "INFRAKIT_VSPHERE_VCHOST"
)

var (
	log = logutil.New("module", "run/v0/vsphere")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	// TODO - this is currently implemented in AWS and other cloud providers but not
	// in vsphere
	Namespace map[string]string

	// IgnoreOnDestroy sets the behavior on destroying instances
	IgnoreOnDestroy bool

	// VCenters is a list of VCenters - each corresponds to config of a plugin instance
	VCenters []vsphere.Options
}

func defaultNamespace() map[string]string {
	t := map[string]string{}
	list := local.Getenv(EnvNamespaceTags, "")
	for _, v := range strings.Split(list, ",") {
		p := strings.Split(v, "=")
		if len(p) == 2 {
			t[p[0]] = p[1]
		}
	}
	return t
}

func defaultVCenterOptions() vsphere.Options {
	return vsphere.Options{
		VCenterURL:  local.Getenv(EnvVCURL, "https://username:password@VCaddress/sdk"),
		DataCenter:  local.Getenv(EnvVCDataCenter, ""),
		DataStore:   local.Getenv(EnvVCDataStore, ""),
		NetworkName: local.Getenv(EnvVCNetwork, ""),
		VSphereHost: local.Getenv(EnvVCHost, ""),
	}
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Namespace: defaultNamespace(),
	VCenters: []vsphere.Options{
		defaultVCenterOptions(),
	},
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	hypervisors := map[string]instance.Plugin{}

	for _, vcenter := range options.VCenters {
		// Assign either an alias or the initial vSphere hosts to differentiate the plugins
		vcenterName := local.Getenv(EnvVCAlias, EnvVCHost)
		hypervisors[vcenterName] = vsphere.NewInstancePlugin(vcenter)
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: hypervisors,
	}
	return
}
