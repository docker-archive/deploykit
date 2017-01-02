package template

import (
	"bytes"
	"strings"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig"
)

// Options contains parameters for customizing the behavior of the engine
type Options struct {

	// SocketDir is the directory for locating the socket file for
	// a template URL of the form unix://socket_file/path/to/resource
	SocketDir string
}

// Template is the templating engine
type Template struct {
	options Options

	url    string
	body   []byte
	parsed *template.Template
	funcs  map[string]interface{}
	binds  map[string]interface{}
	lock   sync.Mutex
}

// NewTemplate fetches the content at the url and returns a template
func NewTemplate(s string, opt Options) (*Template, error) {
	var buff []byte
	// Special case of specifying the entire template as a string; otherwise treat as url
	if strings.Index(s, "str://") == 0 {
		buff = []byte(strings.Replace(s, "str://", "", 1))
	} else {
		b, err := fetch(s, opt)
		if err != nil {
			return nil, err
		}
		buff = b
	}

	return &Template{
		options: opt,
		url:     s,
		body:    buff,
		funcs:   map[string]interface{}{},
		binds:   map[string]interface{}{},
	}, nil
}

// SetOptions sets the runtime flags for the engine
func (t *Template) SetOptions(opt Options) *Template {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.options = opt
	return t
}

// AddFunc adds a new function to support in template
func (t *Template) AddFunc(name string, f interface{}) *Template {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.funcs[name] = f
	return t
}

func (t *Template) build() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.parsed != nil {
		return nil
	}

	fm := t.DefaultFuncs()

	for k, v := range sprig.TxtFuncMap() {
		fm[k] = v
	}

	for k, v := range t.funcs {
		fm[k] = v
	}

	parsed, err := template.New(t.url).Funcs(fm).Parse(string(t.body))
	if err != nil {
		return err
	}

	t.parsed = parsed
	return nil
}

// Render renders the template given the context
func (t *Template) Render(context interface{}) (string, error) {
	if err := t.build(); err != nil {
		return "", err
	}
	var buff bytes.Buffer
	err := t.parsed.Execute(&buff, context)
	return buff.String(), err
}
