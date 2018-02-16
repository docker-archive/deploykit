package terraform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	terraform "github.com/docker/infrakit/pkg/provider/terraform/instance"
	terraform_types "github.com/docker/infrakit/pkg/provider/terraform/instance/types"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "terraform"

	// EnvDir is the env for directory for file storage
	EnvDir = "INFRAKIT_INSTANCE_TERRAFORM_DIR"
)

var (
	log = logutil.New("module", "run/v0/terraform")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// DefaultOptions return an Options with default values filled in.  If you want to expose these to the CLI,
// simply get this struct and bind the fields to the flags.
var DefaultOptions = terraform_types.Options{
	Dir:          local.Getenv(EnvDir, filepath.Join(local.InfrakitHome(), "terraform")),
	PollInterval: types.FromDuration(30 * time.Second),
	Standalone:   false,
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

	os.MkdirAll(options.Dir, 0755)

	err = mustHaveTerraform()
	if err != nil {
		return
	}

	importInstSpec, err := options.ParseInstanceSpecFromGroup(scope)
	if err != nil {
		// If we cannot parse the group spec then we cannot import the resource, the plugin should
		// not start since terraform is not managing the resource
		log.Error("error parsing instance spec from group", "err", err)
		return
	}

	// Do we have the new options?
	log.Info("NewOptions", "value", options.NewOption, "Dir", options.Dir)

	// Parse import options
	resources := []*terraform.ImportResource{}
	for _, importResource := range options.ImportResources {
		resType := terraform.TResourceType(importResource.ResourceType)
		resName := terraform.TResourceName(importResource.ResourceName)
		resID := importResource.ResourceID
		excludePropIDs := importResource.ExcludePropIDs
		res := terraform.ImportResource{
			ResourceType:   &resType,
			ResourceName:   &resName,
			ResourceID:     &resID,
			ExcludePropIDs: &excludePropIDs,
		}
		resources = append(resources, &res)
	}
	plugin, err := terraform.NewTerraformInstancePlugin(options,
		&terraform.ImportOptions{
			InstanceSpec: importInstSpec,
			Resources:    resources,
		},
	)
	if err != nil {
		log.Error("error initializing pluing", "err", err)
		return
	}
	impls = map[run.PluginCode]interface{}{
		run.Instance: plugin,
	}
	transport.Name = name
	return
}

func mustHaveTerraform() error {
	// check if terraform exists
	if _, err := exec.LookPath("terraform"); err != nil {
		log.Crit("Cannot find terraform.  Please install at https://www.terraform.io/downloads.html")
		return fmt.Errorf("cannot find terraform")
	}
	return nil
}
