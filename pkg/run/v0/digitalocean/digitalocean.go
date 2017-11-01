package digitalocean

import (
	"context"
	"strings"

	"github.com/digitalocean/godo"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	digitalocean "github.com/docker/infrakit/pkg/provider/digitalocean/plugin/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"golang.org/x/oauth2"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "digitalocean"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_DIGITALOCEAN_NAMESPACE_TAGS"

	// EnvAccessToken is the access token
	EnvAccessToken = "INFRAKIT_DIGITALOCEAN_ACCESS_TOKEN"
)

var (
	log = logutil.New("module", "run/v0/digitalocean")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	Namespace map[string]string

	// AccessToken is the OAuth access token
	AccessToken string
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
	AccessToken: local.Getenv(EnvAccessToken, ""),
	Namespace:   defaultNamespace(),
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

	token := &oauth2.Token{AccessToken: options.AccessToken}
	tokenSource := oauth2.StaticTokenSource(token)
	oauthClient := oauth2.NewClient(context.TODO(), tokenSource)
	client := godo.NewClient(oauthClient)

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: map[string]instance.Plugin{
			"compute": digitalocean.NewDOInstancePlugin(client, options.Namespace),
		},
	}
	return
}
