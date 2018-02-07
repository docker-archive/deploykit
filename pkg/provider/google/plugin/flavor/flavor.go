package flavor

import (
	"errors"
	"log"
	"strings"
	"time"

	group_controller "github.com/docker/infrakit/pkg/controller/group"
	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	"github.com/docker/infrakit/pkg/provider/google/plugin/gcloud"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// Spec is the model of the plugin Properties.
type Spec struct {
	Flavors []group_types.FlavorPlugin
}

type flavorCombo struct {
	API           gcloud.API
	flavorPlugins group_controller.FlavorPluginLookup
	minAge        time.Duration
}

// NewPlugin creates a Flavor Combo plugin that chains multiple flavors in a sequence.
func NewPlugin(flavorPlugins group_controller.FlavorPluginLookup, project, zone string, minAge time.Duration) flavor.Plugin {
	api, err := gcloud.NewAPI(project, zone)
	if err != nil {
		log.Fatal(err)
	}

	return flavorCombo{
		API:           api,
		flavorPlugins: flavorPlugins,
		minAge:        minAge,
	}
}

func (f flavorCombo) Validate(flavorProperties *types.Any, allocation group.AllocationMethod) error {
	s := Spec{}
	return flavorProperties.Decode(&s)
}

func (f flavorCombo) Healthy(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
	name := string(inst.ID)

	instance, err := f.API.GetInstance(name)
	if err != nil {
		return flavor.Unknown, err
	}

	switch instance.Status {
	case "STOPPED", "STOPPING", "SUSPENDED", "SUSPENDING", "TERMINATED":
		return flavor.Unhealthy, nil
	case "RUNNING":
		if f.minAge == 0 {
			return flavor.Healthy, nil
		}

		creation, err := time.Parse(time.RFC3339, instance.CreationTimestamp)
		if err != nil {
			return flavor.Unknown, err
		}

		if creation.Add(f.minAge).Before(time.Now()) {
			return flavor.Healthy, nil
		}
		return flavor.Unknown, nil
	case "PROVISIONING", "STAGING":
		return flavor.Unknown, nil
	}

	return flavor.Unknown, nil
}

func (f flavorCombo) Drain(flavorProperties *types.Any, inst instance.Description) error {
	// Draining is attempted on all flavors regardless of errors encountered.  All errors encountered are combined
	// and returned.

	s := Spec{}
	if err := flavorProperties.Decode(&s); err != nil {
		return err
	}

	errs := []string{}

	for _, pluginSpec := range s.Flavors {
		plugin, err := f.flavorPlugins(pluginSpec.Plugin)
		if err != nil {
			errs = append(errs, err.Error())
		}

		if err := plugin.Drain(pluginSpec.Properties, inst); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errors.New(strings.Join(errs, ", "))
}

func cloneSpec(spec instance.Spec) instance.Spec {
	tags := map[string]string{}
	for k, v := range spec.Tags {
		tags[k] = v
	}

	var logicalID *instance.LogicalID
	if spec.LogicalID != nil {
		idCopy := *spec.LogicalID
		logicalID = &idCopy
	}

	attachments := []instance.Attachment{}
	for _, v := range spec.Attachments {
		attachments = append(attachments, v)
	}

	return instance.Spec{
		Properties:  spec.Properties,
		Tags:        tags,
		Init:        spec.Init,
		LogicalID:   logicalID,
		Attachments: attachments,
	}
}

func mergeSpecs(initial instance.Spec, specs []instance.Spec) (instance.Spec, error) {
	result := cloneSpec(initial)

	for _, spec := range specs {
		for k, v := range spec.Tags {
			result.Tags[k] = v
		}

		if spec.Init != "" {
			if result.Init != "" {
				result.Init += "\n"
			}

			result.Init += spec.Init
		}

		for _, v := range spec.Attachments {
			result.Attachments = append(result.Attachments, v)
		}
	}

	return result, nil
}

func (f flavorCombo) Prepare(flavor *types.Any, inst instance.Spec, allocation group.AllocationMethod,
	context group.Index) (instance.Spec, error) {

	combo := Spec{}
	err := flavor.Decode(&combo)
	if err != nil {
		return inst, err
	}

	specs := []instance.Spec{}
	for _, pluginSpec := range combo.Flavors {
		// Copy the instance spec to prevent Flavor plugins from interfering with each other.
		clone := cloneSpec(inst)

		plugin, err := f.flavorPlugins(pluginSpec.Plugin)
		if err != nil {
			return inst, err
		}

		output, err := plugin.Prepare(pluginSpec.Properties, clone, allocation, context)
		if err != nil {
			return inst, err
		}
		specs = append(specs, output)
	}

	return mergeSpecs(inst, specs)
}
