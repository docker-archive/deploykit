package libmachete

import (
	"fmt"
	"github.com/docker/libmachete/storage"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
)

// KVPair models the key-value pairs
type KVPair map[string]interface{}

type ContextBuilder func(context.Context, KVPair) context.Context

var (
	contextBuilders = map[string]ContextBuilder{}
)

// RegisterCredentialer registers the function that allocates an empty credential for a provisioner.
// This method should be invoke in the init() of the provisioner package.
func RegisterContextBuilder(provisionerName string, f ContextBuilder) {
	lock.Lock()
	defer lock.Unlock()

	contextBuilders[provisionerName] = f
}

// BuildContext builds the runtime context customized for the provisioner
func BuildContext(provisionerName string, root context.Context, ctx Context) context.Context {
	builder, has := contextBuilders[provisionerName]
	if !has {
		return root
	}
	// get the NVPair by the provisioner name
	nvpair, has := ctx[provisionerName]
	if !has {
		return root
	}
	return builder(root, nvpair)
}

const (
	ErrContextDuplicate int = iota
	ErrContextNotFound
)

type ContextError struct {
	Code    int
	Message string
}

func (e ContextError) Error() string {
	return e.Message
}

// Context is the application / provisioner-level configuration object that
// stores config parameters like S3 bucket storage, timeout, retries, etc.
// The map keys can be provisioner names, in which case, the value object is
// passed to the provisioner to process. For example, in YAML:
//
// # context
// storage_bucket : ... # S3 bucket here
// aws:  # AWS specific
//    region: us-west-2
//    retries: -1
// azure:  # Azure
//    client_id: 123455656 # OAuth client id
//    subscription_id: 123456 # User account / subscription id
type Context map[string]KVPair

// Contexts looks up and reads context data, scoped by provisioner name.
type Contexts interface {

	// Unmarshal decodes the bytes and applies onto the object, with a encoding.
	// If nil codec is passed, the default encoding / content type will be used.
	Unmarshal(contentType *Codec, data []byte, ctx *Context) error

	// Marshal encodes the given context object and returns the bytes.
	// If nil codec is passed, the default encoding / content type will be used.
	Marshal(contentType *Codec, ctx Context) ([]byte, error)

	// ListIds
	ListIds() ([]string, error)

	// Saves the context identified by key
	Save(key string, ctx Context) error

	// Get returns a context identified by key
	Get(key string) (Context, error)

	// Deletes the context identified by key
	Delete(key string) error

	// Exists returns true if context identified key exists
	Exists(key string) bool

	// CreateContext adds a new context from the input reader.
	CreateContext(key string, input io.Reader, codec *Codec) *ContextError

	// UpdateContext updates an existing context
	UpdateContext(key string, input io.Reader, codec *Codec) *ContextError
}

type contexts struct {
	store storage.Contexts
}

// NewContexts creates an instance of the manager given the backing store.
func NewContexts(store storage.Contexts) Contexts {
	return &contexts{store: store}
}

// Unmarshal decodes the bytes and applies onto the context object, using a given encoding.
// If nil codec is passed, the default encoding / content type will be used.
func (t *contexts) Unmarshal(contentType *Codec, data []byte, ctx *Context) error {
	return ensureValidContentType(contentType).unmarshal(data, ctx)
}

// Marshal encodes the given context object and returns the bytes.
// If nil codec is passed, the default encoding / content type will be used.
func (t *contexts) Marshal(contentType *Codec, ctx Context) ([]byte, error) {
	return ensureValidContentType(contentType).marshal(ctx)
}

func (t *contexts) ListIds() ([]string, error) {
	list := []string{}
	ids, err := t.store.List()
	if err != nil {
		return nil, err
	}
	for _, i := range ids {
		list = append(list, string(i))
	}
	return list, nil
}

func (t *contexts) Save(key string, ctx Context) error {
	return t.store.Save(storage.ContextID(key), ctx)
}

func (t *contexts) Get(key string) (Context, error) {
	ctx := new(Context)
	err := t.store.GetContext(storage.ContextID(key), ctx)
	if err != nil {
		return nil, err
	}
	return *ctx, nil
}

func (t *contexts) Delete(key string) error {
	return t.store.Delete(storage.ContextID(key))
}

func (t *contexts) Exists(key string) bool {
	ctx := new(Context)
	err := t.store.GetContext(storage.ContextID(key), ctx)
	return err == nil
}

// CreateContext creates a new context from the input reader.
func (t *contexts) CreateContext(key string, input io.Reader, codec *Codec) *ContextError {
	if t.Exists(key) {
		return &ContextError{ErrContextDuplicate, fmt.Sprintf("Key exists: %v", key)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &ContextError{Message: err.Error()}
	}

	ctx := new(Context)
	if err = t.Unmarshal(codec, buff, ctx); err != nil {
		return &ContextError{Message: err.Error()}
	}
	if err = t.Save(key, *ctx); err != nil {
		return &ContextError{Message: err.Error()}
	}
	return nil
}

func (t *contexts) UpdateContext(key string, input io.Reader, codec *Codec) *ContextError {
	if !t.Exists(key) {
		return &ContextError{ErrContextNotFound, fmt.Sprintf("Context not found: %v", key)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &ContextError{Message: err.Error()}

	}

	ctx := new(Context)
	if err = t.Unmarshal(codec, buff, ctx); err != nil {
		return &ContextError{Message: err.Error()}
	}
	if err = t.Save(key, *ctx); err != nil {
		return &ContextError{Message: err.Error()}
	}
	return nil
}
