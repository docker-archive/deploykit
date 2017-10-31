package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	terraform "github.com/docker/infrakit/pkg/provider/terraform/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
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

// ImportResourceOptions defines a resource to import
type ImportResourceOptions struct {
	// Terraform resource type
	ResourceType string

	// Resource name in the group spec
	ResourceName string

	// ID of the resource to import
	ResourceID string

	// IDs of the properties to exclude from the instance spec
	ExcludePropIDs []string
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Dir for storing plan files
	Dir string

	// PollInterval is the Terraform polling interval
	PollInterval types.Duration

	// Standalone - set if running standalone, disables manager leadership verification
	Standalone bool

	// ImportGroupSpecURL defines the group spec that the instance is imported into.
	ImportGroupSpecURL string

	// ImportResources defines the instances to import
	ImportResources []ImportResourceOptions

	// ImportGroupID defines the group ID to import the resource into (optional)
	ImportGroupID string

	// NewOption is an example... see the plugins.json file in this directory.
	NewOption string

	// Envs are the environment variables to include when invoking terraform
	Envs types.Any
}

// DefaultOptions return an Options with default values filled in.  If you want to expose these to the CLI,
// simply get this struct and bind the fields to the flags.
var DefaultOptions = Options{
	Dir:          local.Getenv(EnvDir, filepath.Join(local.InfrakitHome(), "terraform")),
	PollInterval: types.FromDuration(30 * time.Second),
	Standalone:   false,
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins, name plugin.Name,
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

	importInstSpec, err := parseInstanceSpecFromGroup(options.ImportGroupSpecURL, options.ImportGroupID)
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
	// Environment varables to include when invoking terraform
	envs, err := parseOptionsEnvs(&options.Envs)
	if err != nil {
		log.Error("error parsing configuration Env Options", "err", err)
		return
	}
	impls = map[run.PluginCode]interface{}{
		run.Instance: terraform.NewTerraformInstancePlugin(options.Dir, options.PollInterval.Duration(),
			options.Standalone, envs, &terraform.ImportOptions{
				InstanceSpec: importInstSpec,
				Resources:    resources,
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
	// Use the first logical ID if set
	if len(groupProps.Allocation.LogicalIDs) > 0 {
		tags["LogicalID"] = string(groupProps.Allocation.LogicalIDs[0])
	}

	spec := instance.Spec{
		Properties: groupProps.Instance.Properties,
		Tags:       tags,
	}
	log.Info("Successfully processed instance spec from group.", "group", groupSpec.ID, "spec", spec)

	return &spec, nil
}

// parseOptionsEnvs processes the data to create a key=value slice of strings
func parseOptionsEnvs(data *types.Any) ([]string, error) {
	envs := []string{}
	if data == nil || len(data.Bytes()) == 0 {
		return envs, nil
	}
	err := json.Unmarshal(data.Bytes(), &envs)
	if err != nil {
		return envs, fmt.Errorf("Failed to unmarshall Options.Envs data: %v", err)
	}
	// Must be key=value pairs
	for _, val := range envs {
		if !strings.Contains(val, "=") {
			return []string{}, fmt.Errorf("Env var is missing '=' character: %v", val)
		}
	}
	return envs, err
}
