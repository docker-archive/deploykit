package filestores

import (
	"github.com/docker/libmachete/storage"
)

type templates struct {
	sandbox Sandbox
}

// NewTemplates creates a new templates store backed by the local file system.
func NewTemplates(sandbox Sandbox) storage.Templates {
	return &templates{sandbox: sandbox}
}

func (c templates) Save(id storage.TemplateID, templatesData interface{}) error {
	return c.sandbox.marshalAndSave(id.Key(), templatesData)
}

func (c templates) List() ([]storage.TemplateID, error) {
	contents, err := c.sandbox.list()
	if err != nil {
		return nil, err
	}
	ids := []storage.TemplateID{}
	for _, element := range contents {
		ids = append(ids, storage.TemplateIDFromString(element))
	}
	return ids, nil
}

// TODO(wfarner): Consider pushing unmarshaling higher up the stack.  At the very least, other store implementations
// should not need to reimplement this.
func (c templates) GetTemplate(id storage.TemplateID, templatesData interface{}) error {
	return c.sandbox.readAndUnmarshal(id.Key(), templatesData)
}

func (c templates) Delete(id storage.TemplateID) error {
	return c.sandbox.remove(id.Key())
}
