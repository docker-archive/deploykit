package maas

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	maas "github.com/docker/infrakit/pkg/provider/maas/plugin/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "maas"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_MAAS_NAMESPACE_TAGS"

	// EnvDir is the MAAS directory
	EnvDir = "INFRAKIT_MAAS_DIR"

	// EnvAPIKey is the env to set the API key
	EnvAPIKey = "INFRAKIT_MAAS_API_KEY"

	// EnvURL is the env to set the connection url
	EnvURL = "INFRAKIT_MAAS_URL"

	// EnvAPIVersion is the env to set the API version
	EnvAPIVersion = "INFRAKIT_MAAS_API_VERSION"
)

var (
	log = logutil.New("module", "run/v0/maas")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	// TODO support this
	Namespace map[string]string

	// Dir is the MAAS directory
	Dir string

	// APIKey is the API token
	APIKey string

	// APIVersion is the version of the MAAS API
	APIVersion string

	// URL to connect to MAAS
	URL string
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

func defaultDir() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return dir
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Namespace:  defaultNamespace(),
	APIKey:     local.Getenv(EnvAPIKey, "aaaa:bbbb:ccccc"),
	APIVersion: local.Getenv(EnvAPIVersion, "2.0"),
	Dir:        local.Getenv(EnvDir, defaultDir()),
	URL:        local.Getenv(EnvURL, "127.0.0.1:80"),
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

	if options.APIVersion == "1.0" {
		err = fmt.Errorf("MAAS API version 1.0 is no longer supported.  Use 2.0")
		return
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: maas.NewMaasPlugin(options.Dir, options.APIKey, options.URL, options.APIVersion),
	}
	return
}
