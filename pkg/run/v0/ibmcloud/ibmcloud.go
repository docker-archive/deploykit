package ibmcloud

import (
	"fmt"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	ibmcloud_auth_inst "github.com/docker/infrakit/pkg/provider/ibmcloud/plugin/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "ibmcloud"

	// EnvIBMCloudUsername is the name of the LB ENV variable name for the IBM Cloud Username.
	EnvIBMCloudUsername = "INFRAKIT_IBMCLOUD_USERNAME"

	// EnvIBMCloudAPIKey is the name of the LB ENV variable name for the IBM Cloud API Key.
	EnvIBMCloudAPIKey = "INFRAKIT_IBMCLOUD_APIKEY"
)

var (
	log = logutil.New("module", "run/v0/ibmcloud")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// VolumeAuth is the type that contains the volume information to authorize
type VolumeAuth struct {
	// VolumeID is the volume to authorize to the group members
	VolumeID int
}

// Options capture the options for starting up the plugin.
type Options struct {
	Username   string
	APIKey     string
	VolumeAuth VolumeAuth
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Username: local.Getenv(EnvIBMCloudUsername, ""),
	APIKey:   local.Getenv(EnvIBMCloudAPIKey, ""),
	VolumeAuth: VolumeAuth{
		VolumeID: 0,
	},
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {
	log.Debug("Run", "Name", name)

	options := Options{}
	err = config.Decode(&options)
	if err != nil {
		return
	}
	if options.Username == "" || options.APIKey == "" {
		err = fmt.Errorf("IBM Cloud username and APIKey required")
		return
	}

	var authInstPlugin instance.Plugin
	if options.VolumeAuth.VolumeID != 0 {
		authInstPlugin = ibmcloud_auth_inst.NewVolumeAuthPlugin(options.Username, options.APIKey, options.VolumeAuth.VolumeID)
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{}
	if authInstPlugin != nil {
		impls[run.Instance] = map[string]instance.Plugin{"instance-vol-auth": authInstPlugin}
	}

	return
}
