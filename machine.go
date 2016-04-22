package libmachete

import (
	"fmt"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v2"
)

// Machine interfaces between provisioner-specific machine templates and provisioners to create
// instances.
type machine struct {
	registry       *Registry
	templateLoader func(provisioner string, template string) ([]byte, error)
}

// CreateMachine creates a new machine.
func (m *machine) CreateMachine(
	provisionerName string,
	provisionerParams map[string]string,
	templateName string,
	overrideData []byte) (<-chan api.CreateInstanceEvent, error) {

	provisioner := m.registry.Get(provisionerName, provisionerParams)
	if provisioner == nil {
		return nil, fmt.Errorf("Provisioner '%s' does not exist.", provisionerName)
	}

	templateData, err := m.templateLoader(provisionerName, templateName)
	if err != nil {
		return nil, fmt.Errorf("Failed to load template '%s': %s", templateName, err)
	}

	base := provisioner.NewRequestInstance()
	err = yaml.Unmarshal(templateData, base)
	if err != nil {
		return nil, fmt.Errorf("Template '%s' is invalid: %s", templateName, err)
	}

	overlay := provisioner.NewRequestInstance()
	err = yaml.Unmarshal(overrideData, overlay)
	if err != nil {
		return nil, fmt.Errorf("Template parameters are invalid: %s", err)
	}

	// Overlay the parameters onto the template.
	err = mergo.MergeWithOverwrite(base, overlay)
	if err != nil {
		return nil, fmt.Errorf("Failed to apply parameters to template: %s", err)
	}

	return provisioner.CreateInstance(base)
}
