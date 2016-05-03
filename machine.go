package libmachete

import (
	"fmt"
	"github.com/docker/libmachete/provisioners"
	"github.com/docker/libmachete/provisioners/api"
	"gopkg.in/yaml.v2"
)

// MachineCreator creates machines with provisioners using instructions provided in template files.
type MachineCreator interface {
	Create(
		provisionerName string,
		provisionerParams map[string]string,
		templateName string,
		overrideData []byte) (<-chan api.CreateInstanceEvent, error)
}

// Machine interfaces between provisioner-specific machine templates and provisioners to create
// instances.
type machine struct {
	registry       *provisioners.Registry
	templateLoader Templates
}

// NewCreator creates a machine creator that will use the provided registry and templates.
func NewCreator(registry *provisioners.Registry, templates Templates) MachineCreator {
	return &machine{registry: registry, templateLoader: templates}
}

// CreateMachine creates a new machine.
func (m *machine) Create(
	provisionerName string,
	provisionerParams map[string]string,
	templateName string,
	overrideData []byte) (<-chan api.CreateInstanceEvent, error) {

	provisioner, err := m.registry.Get(provisionerName, provisionerParams)
	if err != nil {
		return nil, err
	}

	templateData, err := m.templateLoader.Read(provisionerName, templateName)
	if err != nil {
		return nil, fmt.Errorf("Failed to load template '%s': %s", templateName, err)
	}

	base := provisioner.NewRequestInstance()
	err = yaml.Unmarshal(templateData, base)
	if err != nil {
		return nil, fmt.Errorf("Template '%s' is invalid: %s", templateName, err)
	}

	err = yaml.Unmarshal(overrideData, base)
	if err != nil {
		return nil, fmt.Errorf("Template parameters are invalid: %s", err)
	}

	return provisioner.CreateInstance(base)
}
