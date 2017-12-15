package docker

import (
	"strings"
	"time"

	apitypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/metadata"
	docker "github.com/docker/infrakit/pkg/provider/docker/plugin/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
	"golang.org/x/net/context"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "docker"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_DOCKER_NAMESPACE_TAGS"

	// EnvURIs is the env to set the list of connection URI.  The format
	// is name1=uri1,name2=uri2,...
	EnvURIs = "INFRAKIT_DOCKER_URIS"
)

var (
	log = logutil.New("module", "run/v0/docker")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	Namespace map[string]string
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

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Namespace: defaultNamespace(),
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

	builder := docker.Builder{}

	// channel for metadata update
	updateSnapshot := make(chan func(map[string]interface{}))
	// channel for metadata update stop signal
	stopSnapshot := make(chan struct{})

	onStop = func() { close(stopSnapshot) }

	instancePlugin, e := builder.BuildInstancePlugin(options.Namespace)
	if e != nil {
		err = e
		return
	}

	// filter on containers managed by InfraKit
	filter := filters.NewArgs()
	filter.Add("label", group.GroupTag)
	listOptions := apitypes.ContainerListOptions{Filters: filter}
	go func() {
		tick := time.Tick(2 * time.Second)
		for {
			select {
			case <-tick:
				snapshot := map[string]interface{}{}
				// list all the containers, inspect them and add it to the snapshot
				ctx := context.Background()
				containers, e := builder.DockerClient().ContainerList(ctx, listOptions)
				if e != nil {
					err = e
					log.Warn("Metadata update failed to list containers", "err", err)
					snapshot["err"] = err
					continue
				}
				for _, container := range containers {
					cid := container.ID
					if json, e := builder.DockerClient().ContainerInspect(ctx, cid); err == nil {
						snapshot[cid] = json
					} else {
						err = e
						log.Warn("Failed to get metadata for container", "cid", cid, "err", err)
						snapshot["err"] = err
					}
				}
				updateSnapshot <- func(view map[string]interface{}) {
					types.Put([]string{"containers"}, snapshot, view)
				}

			case <-stopSnapshot:
				log.Info("Snapshot updater stopped")
				return
			}
		}
	}()

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Metadata: metadata.NewPluginFromChannel(updateSnapshot),
		run.Instance: instancePlugin,
	}
	return
}
