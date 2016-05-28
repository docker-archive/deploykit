package libmachete

import (
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
	ListIds() ([]storage.TemplateID, *Error)

	// Get returns a template identified by provisioner and key
	Get(id storage.TemplateID) (api.MachineRequest, *Error)

	// Deletes the template identified by provisioner and key
	Delete(id storage.TemplateID) *Error

	// CreateTemplate adds a new template from the input reader.
	CreateTemplate(id storage.TemplateID, input io.Reader, codec *Codec) *Error

	// UpdateTemplate updates an existing template
	UpdateTemplate(id storage.TemplateID, input io.Reader, codec *Codec) *Error
}

type templates struct {
	store        storage.Templates
	provisioners *MachineProvisioners
}

// NewTemplates creates an instance of the manager given the backing store.
func NewTemplates(store storage.Templates, provisioners *MachineProvisioners) Templates {
	return &templates{store: store, provisioners: provisioners}
}

func (t *templates) NewBlankTemplate(provisionerName string) (api.MachineRequest, *Error) {
	if builder, has := t.provisioners.GetBuilder(provisionerName); has {
		return builder.DefaultMachineRequest(), nil
	}
	return nil, &Error{ErrBadInput, fmt.Sprintf("Unknown provisioner: %v", provisionerName)}
}

func (t *templates) ListIds() ([]storage.TemplateID, *Error) {
	ids, err := t.store.List()
	if err != nil {
		return nil, UnknownError(err)
	}
	return ids, nil
}

func (t *templates) Get(id storage.TemplateID) (api.MachineRequest, *Error) {
	detail, apiErr := t.NewBlankTemplate(id.Provisioner)
	if apiErr != nil {
		return nil, apiErr
	}

	err := t.store.GetTemplate(id, detail)
	if err != nil {
		return nil, &Error{ErrNotFound, err.Error()}
	}
	return detail, nil
}

func (t *templates) Delete(id storage.TemplateID) *Error {
	err := t.store.Delete(id)
	if err != nil {
		return &Error{ErrNotFound, err.Error()}
	}
	return nil
}

func (t *templates) exists(id storage.TemplateID) bool {
	tmpl, err := t.NewBlankTemplate(id.Provisioner)
	if err != nil {
		return false
	}

	// TODO(wfarner): This result mixes error cases (failure to read/unmarshal with absence).
	return t.store.GetTemplate(id, tmpl) == nil
}

func (t *templates) unmarshal(contentType *Codec, data []byte, tmpl api.MachineRequest) error {
	return contentType.unmarshal(data, tmpl)
}

func (t *templates) CreateTemplate(id storage.TemplateID, input io.Reader, codec *Codec) *Error {
	if t.exists(id) {
		return &Error{ErrDuplicate, fmt.Sprintf("Key exists: %v", id)}
	}

	tmpl, apiErr := t.NewBlankTemplate(id.Provisioner)
	if apiErr != nil {
		return &Error{ErrNotFound, fmt.Sprintf("Unknown provisioner:%s", id.Provisioner)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return UnknownError(err)
	}

	if err = t.unmarshal(codec, buff, tmpl); err != nil {
		return &Error{ErrBadInput, err.Error()}
	}
	if err = t.store.Save(id, tmpl); err != nil {
		return UnknownError(err)
	}
	return nil
}

func (t *templates) UpdateTemplate(id storage.TemplateID, input io.Reader, codec *Codec) *Error {
	if !t.exists(id) {
		return &Error{ErrNotFound, fmt.Sprintf("Template not found: %v", id)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &Error{Message: err.Error()}

	}

	tmpl, apiErr := t.NewBlankTemplate(id.Provisioner)
	if apiErr != nil {
		return apiErr
	}

	if err = t.unmarshal(codec, buff, tmpl); err != nil {
		return &Error{Message: err.Error()}
	}

	if err = t.store.Save(id, tmpl); err != nil {
		return &Error{Message: err.Error()}
	}
	return nil
}
