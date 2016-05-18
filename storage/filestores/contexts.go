package filestores

import (
	"github.com/docker/libmachete/storage"
)

type contexts struct {
	sandbox *sandbox
}

// NewContexts creates a new contexts store backed by the local file system.
func NewContexts(dir string) (storage.Contexts, error) {
	sandbox, err := newSandbox(dir)
	if err != nil {
		return nil, err
	}

	return &contexts{sandbox: sandbox}, nil
}

func (c contexts) Save(id storage.ContextID, contextData interface{}) error {
	return c.sandbox.MarshalAndSave(string(id), contextData)
}

func (c contexts) List() ([]storage.ContextID, error) {
	contents, err := c.sandbox.List()
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
	return c.sandbox.ReadAndUnmarshal(string(id), contextData)
}

func (c contexts) Delete(id storage.ContextID) error {
	return c.sandbox.Remove(string(id))
}
