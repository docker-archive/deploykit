package machines

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/docker/libmachete/storage"
	"io"
	"io/ioutil"
)

// TemplateBuilder is simply a machine request builder, since templates are just
// machine requests with values pre-populated.
type TemplateBuilder MachineRequestBuilder

type templates struct {
	store        storage.KvStore
	provisioners MachineProvisioners
}

// NewTemplates creates an instance of the manager given the backing store.
func NewTemplates(store storage.KvStore, provisioners MachineProvisioners) api.Templates {
	return &templates{store: store, provisioners: provisioners}
}

func (t *templates) NewBlankTemplate(provisionerName string) (spi.MachineRequest, *api.Error) {
	if builder, has := t.provisioners.GetBuilder(provisionerName); has {
		return builder.DefaultMachineRequest(), nil
	}
	return nil, &api.Error{api.ErrBadInput, fmt.Sprintf("Unknown provisioner: %v", provisionerName)}
}

// Key returns the key used for looking up the template.  Key is composed of the provisioner
// name and the name of the template (scoped to a provisioner).
func keyFromTemplateID(id api.TemplateID) storage.Key {
	return storage.Key{Path: []string{id.Provisioner, id.Name}}
}

func templateIDFromKey(key storage.Key) api.TemplateID {
	key.RequirePathLength(2)
	return api.TemplateID{Provisioner: key.Path[0], Name: key.Path[1]}
}

func (t *templates) ListIds() ([]api.TemplateID, *api.Error) {
	keys, err := t.store.ListRecursive(storage.RootKey)
	if err != nil {
		return nil, api.UnknownError(err)
	}

	ids := []api.TemplateID{}
	for _, key := range keys {
		ids = append(ids, templateIDFromKey(key))
	}

	return ids, nil
}

func (t *templates) Get(id api.TemplateID) (spi.MachineRequest, *api.Error) {
	detail, apiErr := t.NewBlankTemplate(id.Provisioner)
	if apiErr != nil {
		return nil, apiErr
	}

	data, err := t.store.Get(keyFromTemplateID(id))
	if err != nil {
		return nil, &api.Error{Code: api.ErrNotFound, Message: "Template does not exist"}
	}

	err = json.Unmarshal(data, detail)
	if err != nil {
		return nil, api.UnknownError(err)
	}

	return detail, nil
}

func (t *templates) Delete(id api.TemplateID) *api.Error {
	err := t.store.Delete(keyFromTemplateID(id))
	if err != nil {
		return &api.Error{api.ErrNotFound, err.Error()}
	}
	return nil
}

func (t *templates) exists(id api.TemplateID) bool {
	_, err := t.store.Get(keyFromTemplateID(id))
	return err == nil
}

func (t *templates) unmarshal(codec api.Codec, data []byte, tmpl spi.MachineRequest) error {
	return codec.Unmarshal(data, tmpl)
}

func (t *templates) saveTemplate(id api.TemplateID, input io.Reader, codec api.Codec) *api.Error {
	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return api.UnknownError(err)

	}

	template, apiErr := t.NewBlankTemplate(id.Provisioner)
	if apiErr != nil {
		return apiErr
	}

	if err = t.unmarshal(codec, buff, template); err != nil {
		return api.UnknownError(err)
	}

	data, err := json.Marshal(template)
	if err != nil {
		return api.UnknownError(err)
	}

	if err = t.store.Save(keyFromTemplateID(id), data); err != nil {
		return api.UnknownError(err)
	}
	return nil
}

func (t *templates) CreateTemplate(id api.TemplateID, input io.Reader, codec api.Codec) *api.Error {
	if t.exists(id) {
		return &api.Error{api.ErrDuplicate, fmt.Sprintf("Key exists: %v", id)}
	}

	return t.saveTemplate(id, input, codec)
}

func (t *templates) UpdateTemplate(id api.TemplateID, input io.Reader, codec api.Codec) *api.Error {
	if !t.exists(id) {
		return &api.Error{api.ErrNotFound, fmt.Sprintf("Template not found: %v", id)}
	}

	return t.saveTemplate(id, input, codec)
}
