package filestores

import (
	"github.com/docker/libmachete/storage"
)

type templates struct {
	sandbox *sandbox
}

// NewTemplates creates a new templates store backed by the local file system.
func NewTemplates(dir string) (storage.Templates, error) {
	sandbox, err := newSandbox(dir)
	if err != nil {
		return nil, err
	}

	return &templates{sandbox: sandbox}, nil
}

func (c templates) Save(id storage.TemplateID, templatesData interface{}) error {
	return c.sandbox.MarshalAndSave(id.Key(), templatesData)
}

func (c templates) List() ([]storage.TemplateID, error) {
	contents, err := c.sandbox.List()
	if err != nil {
		return nil, err
	}
	ids := []storage.TemplateID{}
	for _, element := range contents {
		ids = append(ids, storage.TemplateIDFromString(element))
	}
	return ids, nil
}

func (c templates) GetTemplate(id storage.TemplateID, templatesData interface{}) error {
	return c.sandbox.ReadAndUnmarshal(id.Key(), templatesData)
}

func (c templates) Delete(id storage.TemplateID) error {
	return c.sandbox.Remove(id.Key())
}
