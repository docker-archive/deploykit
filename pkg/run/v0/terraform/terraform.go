package terraform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	terraform "github.com/docker/infrakit/pkg/provider/terraform/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
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

// Options capture the options for starting up the plugin.
type Options struct {
	// Dir for storing plan files
	Dir string

	// PollInterval is the Terraform polling interval
	PollInterval time.Duration

	// Standalone - set if running standalone, disables manager leadership verification
	Standalone bool

	// ImportGroupSpecURL defines the group spec that the instance is imported into.
	ImportGroupSpecURL string

	// ImportInstanceID defines the instance ID to import
	ImportInstanceID string

	// ImportGroupID defines the group ID to import the resource into (optional)
	ImportGroupID string
}

// DefaultOptions return an Options with default values filled in.  If you want to expose these to the CLI,
// simply get this struct and bind the fields to the flags.
var DefaultOptions = Options{
	Dir:          run.GetEnv(EnvDir, filepath.Join(run.InfrakitHome(), "terraform")),
	PollInterval: 30 * time.Second,
	Standalone:   false,
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := Options{}
	err = config.Decode(&options)
	if err != nil {
		return
	}

	os.MkdirAll(options.Dir, 0755)

	err = mustHaveTerraform()
	if err != nil {
		return
	}

	importInstSpec, err := parseInstanceSpecFromGroup(options.ImportGroupSpecURL, options.ImportGroupID)
	if err != nil {
		// If we cannot prase the group spec then we cannot import the resource, the plugin should
		// not start since terraform is not managing the resource
		log.Error("error parsing instance spec from group", "err", err)
		return
	}

	impls = map[run.PluginCode]interface{}{
		run.Instance: terraform.NewTerraformInstancePlugin(options.Dir, options.PollInterval,
			options.Standalone, &terraform.ImportOptions{
				InstanceSpec: importInstSpec,
				InstanceID:   &options.ImportInstanceID,
			}),
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

// parseInstanceSpecFromGroup parses the instance.Spec from the group.Spec and adds
// in the tags that should be set on the imported instance
func parseInstanceSpecFromGroup(groupSpecURL, groupID string) (*instance.Spec, error) {
	// TODO: Support a URL to a manager config with multiple nested groups
	if groupSpecURL == "" {
		log.Info("No group spec URL specified for import")
		return nil, nil
	}
	var groupSpec group.Spec
	t, err := template.NewTemplate(groupSpecURL, template.Options{MultiPass: false})
	if err != nil {
		return nil, err
	}
	template, err := t.Render(nil)
	if err != nil {
		return nil, err
	}
	if err = types.AnyString(template).Decode(&groupSpec); err != nil {
		return nil, err
	}
	// Get the instance properties we care about
	groupProps, err := group_types.ParseProperties(groupSpec)
	if err != nil {
		return nil, err
	}

	// Add in the bootstrap tag and (if set) the group ID
	tags := map[string]string{
		"infrakit.config_sha": "bootstrap",
	}
	// The group ID should match the spec
	if groupID != "" {
		if string(groupSpec.ID) != groupID {
			return nil, fmt.Errorf("Given spec ID '%v' does not match given group ID '%v'",
				string(groupSpec.ID), groupID)
		}
		tags["infrakit.group"] = groupID
	}

	spec := instance.Spec{
		Properties: groupProps.Instance.Properties,
		Tags:       tags,
	}
	log.Info("Successfully processed instance spec from group.", "group", groupSpec.ID, "spec", spec)

	return &spec, nil
}
