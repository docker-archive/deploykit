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
	// NewTemplate creates an instance of the manager given the backing store.
	NewTemplate(provisionerName string) (api.MachineRequest, error)

	// Unmarshal decodes the bytes and applies onto the machine request object, using a given encoding.
	// If nil codec is passed, the default encoding / content type will be used.
	Unmarshal(contentType *Codec, data []byte, cred api.MachineRequest) error

	// Marshal encodes the given template object and returns the bytes.
	// If nil codec is passed, the default encoding / content type will be used.
	Marshal(contentType *Codec, cred api.MachineRequest) ([]byte, error)

	// ListIds
	ListIds() ([]storage.TemplateID, error)

	// Saves the template identified by provisioner and key
	Save(provisioner, key string, cred api.MachineRequest) error

	// Get returns a template identified by provisioner and key
	Get(provisioner, key string) (api.MachineRequest, error)

	// Deletes the template identified by provisioner and key
	Delete(provisioner, key string) error

	// Exists returns true if template identified by provisioner and key already exists
	Exists(provisioner, key string) bool

	// CreateTemplate adds a new template from the input reader.
	CreateTemplate(provisioner, key string, input io.Reader, codec *Codec) *Error

	// UpdateTemplate updates an existing template
	UpdateTemplate(provisioner, key string, input io.Reader, codec *Codec) *Error
}

type templates struct {
	store storage.Templates
}

// NewTemplates creates an instance of the manager given the backing store.
func NewTemplates(store storage.Templates) Templates {
	return &templates{store: store}
}

// NewCredential returns an empty credential object for a provisioner.
func (t *templates) NewTemplate(provisionerName string) (api.MachineRequest, error) {
	if builder, has := GetProvisionerBuilder(provisionerName); has {
		return builder.DefaultMachineRequest, nil
	}
	return nil, fmt.Errorf("Unknown provisioner: %v", provisionerName)
}

// Unmarshal decodes the bytes and applies onto the template object, using a given encoding.
// If nil codec is passed, the default encoding / content type will be used.
func (t *templates) Unmarshal(contentType *Codec, data []byte, tmpl api.MachineRequest) error {
	return ensureValidContentType(contentType).unmarshal(data, tmpl)
}

// Marshal encodes the given template object and returns the bytes.
// If nil codec is passed, the default encoding / content type will be used.
func (t *templates) Marshal(contentType *Codec, tmpl api.MachineRequest) ([]byte, error) {
	return ensureValidContentType(contentType).marshal(tmpl)
}

func (t *templates) ListIds() ([]storage.TemplateID, error) {
	return t.store.List()
}

func (t *templates) Save(provisioner, key string, tmpl api.MachineRequest) error {
	return t.store.Save(storage.TemplateID{provisioner, key}, tmpl)
}

func (t *templates) Get(provisioner, key string) (api.MachineRequest, error) {
	detail, err := t.NewTemplate(provisioner)
	if err != nil {
		return nil, err
	}

	err = t.store.GetTemplate(storage.TemplateID{provisioner, key}, detail)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (t *templates) Delete(provisioner, key string) error {
	return t.store.Delete(storage.TemplateID{provisioner, key})
}

func (t *templates) Exists(provisioner, key string) bool {
	tmpl, err := t.NewTemplate(provisioner)
	if err != nil {
		return false
	}
	err = t.store.GetTemplate(storage.TemplateID{provisioner, key}, tmpl)
	return err == nil
}

// TODO(wfarner): This has no callers, can it be removed?
// CreateTemplate creates a new template from the input reader.
func (t *templates) CreateTemplate(provisioner, key string, input io.Reader, codec *Codec) *Error {
	if t.Exists(provisioner, key) {
		return &Error{ErrDuplicate, fmt.Sprintf("Key exists: %v / %v", provisioner, key)}
	}

	tmpl, err := t.NewTemplate(provisioner)
	if err != nil {
		return &Error{ErrNotFound, fmt.Sprintf("Unknown provisioner:%s", provisioner)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &Error{Message: err.Error()}
	}

	if err = t.Unmarshal(codec, buff, tmpl); err != nil {
		return &Error{Message: err.Error()}
	}
	if err = t.Save(provisioner, key, tmpl); err != nil {
		return &Error{Message: err.Error()}
	}
	return nil
}

func (t *templates) UpdateTemplate(provisioner, key string, input io.Reader, codec *Codec) *Error {
	if !t.Exists(provisioner, key) {
		return &Error{ErrNotFound, fmt.Sprintf("Template not found: %v / %v", provisioner, key)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &Error{Message: err.Error()}

	}

	tmpl, err := t.NewTemplate(provisioner)
	if err != nil {
		return &Error{ErrNotFound, fmt.Sprintf("Unknow provisioner: %v", provisioner)}
	}

	if err = t.Unmarshal(codec, buff, tmpl); err != nil {
		return &Error{Message: err.Error()}
	}

	if err = t.Save(provisioner, key, tmpl); err != nil {
		return &Error{Message: err.Error()}
	}
	return nil
}
