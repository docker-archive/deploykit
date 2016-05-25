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
	NewBlankTemplate(provisionerName string) (api.MachineRequest, error)

	// Unmarshal decodes the bytes and applies onto the machine request object, using a given encoding.
	// If nil codec is passed, the default encoding / content type will be used.
	Unmarshal(contentType *Codec, data []byte, cred api.MachineRequest) error

	// Marshal encodes the given template object and returns the bytes.
	// If nil codec is passed, the default encoding / content type will be used.
	Marshal(contentType *Codec, cred api.MachineRequest) ([]byte, error)

	// ListIds
	ListIds() ([]storage.TemplateID, error)

	// Saves the template identified by provisioner and key
	Save(id storage.TemplateID, cred api.MachineRequest) error

	// Get returns a template identified by provisioner and key
	Get(id storage.TemplateID) (api.MachineRequest, error)

	// Deletes the template identified by provisioner and key
	Delete(id storage.TemplateID) error

	// Exists returns true if template identified by provisioner and key already exists
	Exists(id storage.TemplateID) bool

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

func (t *templates) NewBlankTemplate(provisionerName string) (api.MachineRequest, error) {
	if builder, has := t.provisioners.GetBuilder(provisionerName); has {
		return builder.DefaultMachineRequest, nil
	}
	return nil, fmt.Errorf("Unknown provisioner: %v", provisionerName)
}

// Unmarshal decodes the bytes and applies onto the template object, using a given encoding.
// If nil codec is passed, the default encoding / content type will be used.
func (t *templates) Unmarshal(contentType *Codec, data []byte, tmpl api.MachineRequest) error {
	return contentType.unmarshal(data, tmpl)
}

// Marshal encodes the given template object and returns the bytes.
// If nil codec is passed, the default encoding / content type will be used.
func (t *templates) Marshal(contentType *Codec, tmpl api.MachineRequest) ([]byte, error) {
	return contentType.marshal(tmpl)
}

func (t *templates) ListIds() ([]storage.TemplateID, error) {
	return t.store.List()
}

func (t *templates) Save(id storage.TemplateID, tmpl api.MachineRequest) error {
	return t.store.Save(id, tmpl)
}

func (t *templates) Get(id storage.TemplateID) (api.MachineRequest, error) {
	detail, err := t.NewBlankTemplate(id.Provisioner)
	if err != nil {
		return nil, err
	}

	err = t.store.GetTemplate(id, detail)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (t *templates) Delete(id storage.TemplateID) error {
	return t.store.Delete(id)
}

func (t *templates) Exists(id storage.TemplateID) bool {
	tmpl, err := t.NewBlankTemplate(id.Provisioner)
	if err != nil {
		return false
	}
	err = t.store.GetTemplate(id, tmpl)
	return err == nil
}

func (t *templates) CreateTemplate(id storage.TemplateID, input io.Reader, codec *Codec) *Error {
	if t.Exists(id) {
		return &Error{ErrDuplicate, fmt.Sprintf("Key exists: %v", id)}
	}

	tmpl, err := t.NewBlankTemplate(id.Provisioner)
	if err != nil {
		return &Error{ErrNotFound, fmt.Sprintf("Unknown provisioner:%s", id.Provisioner)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &Error{Message: err.Error()}
	}

	if err = t.Unmarshal(codec, buff, tmpl); err != nil {
		return &Error{Message: err.Error()}
	}
	if err = t.Save(id, tmpl); err != nil {
		return &Error{Message: err.Error()}
	}
	return nil
}

func (t *templates) UpdateTemplate(id storage.TemplateID, input io.Reader, codec *Codec) *Error {
	if !t.Exists(id) {
		return &Error{ErrNotFound, fmt.Sprintf("Template not found: %v", id)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &Error{Message: err.Error()}

	}

	tmpl, err := t.NewBlankTemplate(id.Provisioner)
	if err != nil {
		return &Error{ErrNotFound, fmt.Sprintf("Unknow provisioner: %v", id.Provisioner)}
	}

	if err = t.Unmarshal(codec, buff, tmpl); err != nil {
		return &Error{Message: err.Error()}
	}

	if err = t.Save(id, tmpl); err != nil {
		return &Error{Message: err.Error()}
	}
	return nil
}
