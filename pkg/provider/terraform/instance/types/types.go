package types

import (
	"encoding/json"
	"fmt"
	"strings"

	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/machine/libmachine/log"
)

var (
	logger = logutil.New("module", "provider/terraform/instance/types")
)

// Resource defines a resource to import
type Resource struct {
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
	ImportResources []Resource

	// ImportGroupID defines the group ID to import the resource into (optional)
	ImportGroupID string

	// NewOption is an example... see the plugins.json file in this directory.
	NewOption string

	// Envs are the environment variables to include when invoking terraform
	Envs types.Any
}

// ParseOptionsEnvs processes the data to create a key=value slice of strings
func (o Options) ParseOptionsEnvs() ([]string, error) {
	envs := []string{}
	if o.Envs == nil || len(o.Envs.Bytes()) == 0 {
		return envs, nil
	}
	err := json.Unmarshal(o.Envs.Bytes(), &envs)
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

// ParseInstanceSpecFromGroup parses the instance.Spec from the group.Spec and adds
// in the tags that should be set on the imported instance
func (o Options) ParseInstanceSpecFromGroup(scope scope.Scope) (*instance.Spec, error) {
	if o.ImportGroupSpecURL == "" {
		log.Info("No group spec URL specified for import")
		return nil, nil
	}
	var groupSpec group.Spec
	t, err := scope.TemplateEngine(o.ImportGroupSpecURL, template.Options{MultiPass: false})
	if err != nil {
		logger.Error("ParseInstanceSpecFromGroup",
			"msg", "Failed to create template",
			"spec", o.ImportGroupSpecURL,
			"err", err)
		return nil, err
	}
	template, err := t.Render(nil)
	if err != nil {
		logger.Error("ParseInstanceSpecFromGroup",
			"msg", "Failed to render template",
			"spec", o.ImportGroupSpecURL,
			"err", err)
		return nil, err
	}
	if err = types.AnyString(template).Decode(&groupSpec); err != nil {
		logger.Error("ParseInstanceSpecFromGroup",
			"msg", "Failed to decode template",
			"spec", o.ImportGroupSpecURL,
			"err", err)
		return nil, err
	}
	// Get the instance properties we care about
	groupProps, err := group_types.ParseProperties(groupSpec)
	if err != nil {
		return nil, err
	}

	tags := map[string]string{}
	// The group ID should match the spec
	if o.ImportGroupID != "" {
		if string(groupSpec.ID) != o.ImportGroupID {
			return nil,
				fmt.Errorf("Given spec ID '%v' does not match given group ID '%v'", string(groupSpec.ID), o.ImportGroupID)
		}
		tags[group.GroupTag] = o.ImportGroupID
	}
	// Use the first logical ID if set
	if len(groupProps.Allocation.LogicalIDs) > 0 {
		tags[instance.LogicalIDTag] = string(groupProps.Allocation.LogicalIDs[0])
	}

	spec := instance.Spec{
		Properties: groupProps.Instance.Properties,
		Tags:       tags,
	}
	logger.Info("ParseInstanceSpecFromGroup",
		"msg", "Successfully processed instance spec from group",
		"group", groupSpec.ID,
		"spec", spec)
	return &spec, nil
}
