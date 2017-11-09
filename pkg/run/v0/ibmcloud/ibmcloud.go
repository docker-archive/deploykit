package ibmcloud

import (
	"fmt"
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	ibmcloud_auth_inst "github.com/docker/infrakit/pkg/provider/ibmcloud/plugin/instance"
	ibmcloud_loadbalancer "github.com/docker/infrakit/pkg/provider/ibmcloud/plugin/loadbalancer"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"

	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "ibmcloud"

	// EnvIBMCloudUsername is the name of the LB ENV variable name for the IBM Cloud Username.
	EnvIBMCloudUsername = "INFRAKIT_IBMCLOUD_USERNAME"

	// EnvIBMCloudAPIKey is the name of the LB ENV variable name for the IBM Cloud API Key.
	EnvIBMCloudAPIKey = "INFRAKIT_IBMCLOUD_APIKEY"

	// EnvLBNames is the name of the LB ENV variable name for the array of LB names.
	EnvLBNames = "INFRAKIT_IBMCLOUD_LB_NAMES"

	// EnvLBUUIDs is the name of the LB ENV variable name for the array of LB UUIDS.
	EnvLBUUIDs = "INFRAKIT_IBMCLOUD_LB_UUIDS"
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

// LB is custom type for IBM Cloud loadbalancer definition
type LB struct {
	// LBName is the name of the IBM Cloud loadbalancer
	LBName string

	// LBUUID is the UUID of the IBM Cloud loadbalancer
	LBUUID string
}

func getLBsFromEnv() []LB {
	lbs := []LB{}
	LBNames := strings.Split(local.Getenv(EnvLBNames, ""), ",")
	LBUuids := strings.Split(local.Getenv(EnvLBUUIDs, ""), ",")
	if len(LBNames) == len(LBUuids) {
		for index, name := range LBNames {
			lb := LB{
				LBName: name,
				LBUUID: LBUuids[index],
			}
			lbs = append(lbs, lb)
		}
	}
	return lbs
}

// Options capture the options for starting up the plugin.
type Options struct {
	Username   string
	APIKey     string
	VolumeAuth VolumeAuth
	// LBNames is a list of names for LB instances to start the L4 plugins
	LBs []LB
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Username: local.Getenv(EnvIBMCloudUsername, ""),
	APIKey:   local.Getenv(EnvIBMCloudAPIKey, ""),
	VolumeAuth: VolumeAuth{
		VolumeID: 0,
	},
	LBs: getLBsFromEnv(),
}

// Run runs the plugin, blocking the current thread. Error is returned immediately
// if the plugin cannot be started.
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {
	log.Debug("Run", "Name", name)

	options := Options{}
	err = config.Decode(&options)
	if err != nil {
		log.Error("Error decoding options", "err", err)
		return
	}
	if options.Username == "" || options.APIKey == "" {
		err = fmt.Errorf("IBM Cloud username and APIKey required")
		log.Error("Configuration error", "err", err)
		return
	}

	var authInstPlugin instance.Plugin
	if options.VolumeAuth.VolumeID != 0 {
		authInstPlugin = ibmcloud_auth_inst.NewVolumeAuthPlugin(options.Username, options.APIKey, options.VolumeAuth.VolumeID)
	}

	l4Map := map[string]loadbalancer.L4{}
	for _, lb := range options.LBs {
		var lbPlugin loadbalancer.L4
		lbPlugin, err = ibmcloud_loadbalancer.NewIBMCloudLBPlugin(options.Username, options.APIKey, lb.LBName, lb.LBUUID)
		if err != nil {
			log.Error("Error creating new IBM Cloud LB", "err", err)
			return
		}
		l4Map[lb.LBName] = lbPlugin
	}

	transport.Name = name

	impls = map[run.PluginCode]interface{}{}
	if authInstPlugin != nil {
		impls[run.Instance] = map[string]instance.Plugin{"instance-vol-auth": authInstPlugin}
	}

	if len(options.LBs) > 0 {
		// impls[run.L4] = map[run.PluginCode]interface{}{
		// 	run.L4: func() (map[string]loadbalancer.L4, error) { return l4Map, nil },
		// }
		impls[run.L4] = func() (map[string]loadbalancer.L4, error) { return l4Map, nil }
	}

	return
}
