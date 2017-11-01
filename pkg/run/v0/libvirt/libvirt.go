package libvirt

import (
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	libvirt "github.com/docker/infrakit/pkg/provider/libvirt/plugin/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "libvirt"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_LIBVIRT_NAMESPACE_TAGS"

	// EnvURIs is the env to set the list of connection URI.  The format
	// is name1=uri1,name2=uri2,...
	EnvURIs = "INFRAKIT_LIBVIRT_URIS"
)

var (
	log = logutil.New("module", "run/v0/libvirt")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	// TODO - this is currently implemented in AWS and other cloud providers but not
	// in libvirt
	Namespace map[string]string

	// URIs is a map of name: connection URI
	URIs map[string]string
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

func parseURIs(s string) map[string]string {
	out := map[string]string{}
	for _, p := range strings.Split(s, ",") {
		kv := strings.Split(p, "=")
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Namespace: defaultNamespace(),
	URIs:      parseURIs(local.Getenv(EnvURIs, "default=qemu:///session")),
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

	for name, uri := range options.URIs {
		hypervisors[name] = libvirt.NewLibvirtPlugin(uri)
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: hypervisors,
	}
	return
}
