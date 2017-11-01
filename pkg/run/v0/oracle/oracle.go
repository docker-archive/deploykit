package oracle

import (
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	oracle "github.com/docker/infrakit/pkg/provider/oracle/plugin/loadbalancer"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "oracle"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_ORACLE_NAMESPACE_TAGS"

	// EnvRegion is the env for oracle region.  Don't set this if want auto detect.
	EnvRegion = "INFRAKIT_ORACLE_REGION"

	// EnvStackName is the env for stack name
	EnvStackName = "INFRAKIT_ORACLE_STACKNAME"

	// EnvKeyFile specifies the location of the keyfile
	EnvKeyFile = "INFRAKIT_ORACLE_KEYFILE"

	// EnvFingerprint specifies the fingerprint
	EnvFingerprint = "INFRAKIT_ORACLE_FINGERPRINT"

	// EnvTenancyID specifies the tenancy id
	EnvTenancyID = "INFRAKIT_ORACLE_TENANCY_ID"

	// EnvComponentID specifies the component ID
	EnvComponentID = "INFRAKIT_ORACLE_COMPONENT_ID"

	// EnvUserID specifies the user id
	EnvUserID = "INFRAKIT_ORACLE_USER_ID"

	// EnvLBNames is the name of the LB ENV variable name for the ELB plugin.
	EnvLBNames = "INFRAKIT_ORACLE_LB_NAMES"
)

var (
	log = logutil.New("module", "run/v0/oracle")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	Namespace map[string]string

	// LBNames is a list of names for ELB instances to start the L4 plugins
	LBNames []string

	oracle.Options `json:",inline" yaml:",inline"`
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
	LBNames:   strings.Split(local.Getenv(EnvLBNames, ""), ","),
	Options: oracle.Options{
		UserID:      local.Getenv(EnvUserID, ""),
		ComponentID: local.Getenv(EnvComponentID, ""),
		TenancyID:   local.Getenv(EnvTenancyID, ""),
		Fingerprint: local.Getenv(EnvFingerprint, ""),
		KeyFile:     local.Getenv(EnvKeyFile, ""),
		Region:      local.Getenv(EnvRegion, ""),
		StackName:   local.Getenv(EnvStackName, ""),
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

	apiClient, cErr := oracle.NewClient(&options.Options)
	if cErr != nil {
		err = cErr
		return
	}
	olbClient := oracle.CreateOLBClient(apiClient, &options.Options)

	l4Map := map[string]loadbalancer.L4{}
	for _, name := range options.LBNames {
		var l4 loadbalancer.L4

		l4, err = oracle.NewLoadBalancerDriver(olbClient, name, &options.Options)
		if err != nil {
			return
		}
		l4Map[name] = l4
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.L4: func() (map[string]loadbalancer.L4, error) { return l4Map, nil },
	}

	return
}
