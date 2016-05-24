package filestores

import (
	"github.com/docker/libmachete/storage"
)

type contexts struct {
	sandbox Sandbox
}

// NewContexts creates a new contexts store within the provided sandbox.
func NewContexts(sandbox Sandbox) storage.Contexts {
	return &contexts{sandbox: sandbox}
}

func (c contexts) Save(id storage.ContextID, contextData interface{}) error {
	return c.sandbox.marshalAndSave(string(id), contextData)
}

func (c contexts) List() ([]storage.ContextID, error) {
	contents, err := c.sandbox.list()
	if err != nil {
		return nil, err
	}
	ids := []storage.ContextID{}
	for _, element := range contents {
		ids = append(ids, storage.ContextID(element))
	}
	return ids, nil
}

func (c contexts) GetContext(id storage.ContextID, contextData interface{}) error {
	return c.sandbox.readAndUnmarshal(string(id), contextData)
}

func (c contexts) Delete(id storage.ContextID) error {
	return c.sandbox.remove(string(id))
}
