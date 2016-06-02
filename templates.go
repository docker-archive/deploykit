package libmachete

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"io"
	"io/ioutil"
)

// TemplateBuilder is simply a machine request builder, since templates are just
// machine requests with values pre-populated.
type TemplateBuilder MachineRequestBuilder

// Templates looks up and reads template data, scoped by provisioner name.
type Templates interface {
	// NewTemplate returns a blank template, which can be used to describe the template schema.
	NewBlankTemplate(provisionerName string) (api.MachineRequest, *Error)

	// ListIds
	ListIds() ([]TemplateID, *Error)

	// Get returns a template identified by provisioner and key
	Get(id TemplateID) (api.MachineRequest, *Error)

	// Deletes the template identified by provisioner and key
	Delete(id TemplateID) *Error

	// CreateTemplate adds a new template from the input reader.
	CreateTemplate(id TemplateID, input io.Reader, codec *Codec) *Error

	// UpdateTemplate updates an existing template
	UpdateTemplate(id TemplateID, input io.Reader, codec *Codec) *Error
}

type templates struct {
	store        storage.KvStore
	provisioners *MachineProvisioners
}

// NewTemplates creates an instance of the manager given the backing store.
func NewTemplates(store storage.KvStore, provisioners *MachineProvisioners) Templates {
	return &templates{store: store, provisioners: provisioners}
}

func (t *templates) NewBlankTemplate(provisionerName string) (api.MachineRequest, *Error) {
	if builder, has := t.provisioners.GetBuilder(provisionerName); has {
		return builder.DefaultMachineRequest(), nil
	}
	return nil, &Error{ErrBadInput, fmt.Sprintf("Unknown provisioner: %v", provisionerName)}
}

// Key returns the key used for looking up the template.  Key is composed of the provisioner
// name and the name of the template (scoped to a provisioner).
func keyFromTemplateID(id TemplateID) storage.Key {
	return storage.Key{Path: []string{id.Provisioner, id.Name}}
}

func templateIDFromKey(key storage.Key) TemplateID {
	requirePathLength(key, 2)
	return TemplateID{Provisioner: key.Path[0], Name: key.Path[1]}
}

func (t *templates) ListIds() ([]TemplateID, *Error) {
	keys, err := t.store.ListRecursive(storage.RootKey)
	if err != nil {
		return nil, UnknownError(err)
	}

	ids := []TemplateID{}
	for _, key := range keys {
		ids = append(ids, templateIDFromKey(key))
	}

	return ids, nil
}

func (t *templates) Get(id TemplateID) (api.MachineRequest, *Error) {
	detail, apiErr := t.NewBlankTemplate(id.Provisioner)
	if apiErr != nil {
		return nil, apiErr
	}

	data, err := t.store.Get(keyFromTemplateID(id))
	if err != nil {
		return nil, &Error{Code: ErrNotFound, Message: "Template does not exist"}
	}

	err = json.Unmarshal(data, detail)
	if err != nil {
		return nil, UnknownError(err)
	}

	return detail, nil
}

func (t *templates) Delete(id TemplateID) *Error {
	err := t.store.Delete(keyFromTemplateID(id))
	if err != nil {
		return &Error{ErrNotFound, err.Error()}
	}
	return nil
}

func (t *templates) exists(id TemplateID) bool {
	_, err := t.store.Get(keyFromTemplateID(id))
	return err == nil
}

func (t *templates) unmarshal(contentType *Codec, data []byte, tmpl api.MachineRequest) error {
	return contentType.unmarshal(data, tmpl)
}

func (t *templates) saveTemplate(id TemplateID, input io.Reader, codec *Codec) *Error {
	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return UnknownError(err)

	}

	template, apiErr := t.NewBlankTemplate(id.Provisioner)
	if apiErr != nil {
		return apiErr
	}

	if err = t.unmarshal(codec, buff, template); err != nil {
		return UnknownError(err)
	}

	data, err := json.Marshal(template)
	if err != nil {
		return UnknownError(err)
	}

	if err = t.store.Save(keyFromTemplateID(id), data); err != nil {
		return UnknownError(err)
	}
	return nil
}

func (t *templates) CreateTemplate(id TemplateID, input io.Reader, codec *Codec) *Error {
	if t.exists(id) {
		return &Error{ErrDuplicate, fmt.Sprintf("Key exists: %v", id)}
	}

	return t.saveTemplate(id, input, codec)
}

func (t *templates) UpdateTemplate(id TemplateID, input io.Reader, codec *Codec) *Error {
	if !t.exists(id) {
		return &Error{ErrNotFound, fmt.Sprintf("Template not found: %v", id)}
	}

	return t.saveTemplate(id, input, codec)
}
