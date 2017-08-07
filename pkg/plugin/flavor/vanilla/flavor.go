package vanilla

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// Spec is the model of the Properties section of the top level group spec.
type Spec struct {
	// Init
	Init []string

	// InitScriptTemplateURL provides a URL to a template that is used to generaete Init
	InitScriptTemplateURL string

	// Tags
	Tags map[string]string

	// Attachments are instructions for external entities that should be attached to the instance.
	Attachments []instance.Attachment
}

// NewPlugin creates a Flavor plugin that doesn't do very much. It assumes instances are
// identical (cattles) but can assume specific identities (via the LogicalIDs).  The
// instances here are treated identically because we have constant Init that applies
// to all instances
func NewPlugin(opt template.Options) flavor.Plugin {
	return vanillaFlavor{options: opt}
}

// DefaultOptions contains the default settings.
var DefaultOptions = template.Options{}

type vanillaFlavor struct {
	options template.Options
}

func (f vanillaFlavor) Validate(flavorProperties *types.Any, allocation group_types.AllocationMethod) error {
	spec := Spec{}
	err := flavorProperties.Decode(&spec)
	if err != nil {
		return err
	}
	if spec.InitScriptTemplateURL != "" && len(spec.Init) > 0 {
		return fmt.Errorf("Either \"Init\" or \"InitScriptTemplateURL\" can be specified but not both")
	}

	if spec.InitScriptTemplateURL != "" {
		template, err := template.NewTemplate(spec.InitScriptTemplateURL, f.options)
		if err != nil {
			return err
		}
		_, err = template.Render(nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f vanillaFlavor) Healthy(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
	// TODO: We could add support for shell code in the Spec for a command to run for checking health.
	return flavor.Healthy, nil
}

func (f vanillaFlavor) Drain(flavorProperties *types.Any, inst instance.Description) error {
	// TODO: We could add support for shell code in the Spec for a drain command to run.
	return nil
}

func (f vanillaFlavor) Prepare(flavor *types.Any,
	instance instance.Spec,
	allocation group_types.AllocationMethod,
	index group_types.Index) (instance.Spec, error) {

	s := Spec{}
	err := flavor.Decode(&s)
	if err != nil {
		return instance, err
	}

	// Handle Init lines, either from templated script or raw input; append to
	// and instance.Init lines
	lines := []string{}
	if instance.Init != "" {
		lines = append(lines, instance.Init)
	}
	if s.InitScriptTemplateURL != "" {
		template, err := template.NewTemplate(s.InitScriptTemplateURL, f.options)
		if err != nil {
			return instance, err
		}
		initScript, err := template.Render(nil)
		if err != nil {
			return instance, err
		}
		lines = append(lines, initScript)
		log.Infoln("Init script data:", initScript)
	} else {
		lines = append(lines, s.Init...)
	}

	instance.Init = strings.Join(lines, "\n")

	// Append tags
	for k, v := range s.Tags {
		if instance.Tags == nil {
			instance.Tags = map[string]string{}
		}
		instance.Tags[k] = v
	}

	// Attachements
	for _, a := range s.Attachments {
		instance.Attachments = append(instance.Attachments, a)
	}
	return instance, nil
}
