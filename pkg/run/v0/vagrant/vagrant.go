package vagrant

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	vagrant "github.com/docker/infrakit/pkg/provider/vagrant/plugin/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "vagrant"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_VAGRANT_NAMESPACE_TAGS"

	// EnvDir is the env for setting the vagrant directory
	EnvDir = "INFRAKIT_VAGRANT_DIR"

	// EnvTemplateURL is the env for setting the vagrant file template url
	EnvTemplateURL = "INFRAKIT_VAGRANT_TEMPLATE_URL"
)

var (
	log = logutil.New("module", "run/v0/vagrant")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	// TODO - this is currently implemented in AWS and other cloud providers but not
	// in vagrant
	Namespace map[string]string

	// Dir is the directory where vagrant files are kept
	Dir string

	// TemplateURL is the URL for the vagrant template
	TemplateURL string
}

func defaultNamespace() map[string]string {
	t := map[string]string{}
	list := local.Getenv(EnvNamespaceTags, "")
	for _, v := range strings.Split(list, ",") {
		p := strings.Split(v, "=")
		if len(p) == 2 {
			t[p[0]] = p[1]
		}
	}
	return t
}

func defaultDir() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return dir
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Namespace:   defaultNamespace(),
	Dir:         local.Getenv(EnvDir, ""),
	TemplateURL: local.Getenv(EnvTemplateURL, ""),
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	opts := template.Options{}
	var templ *template.Template
	if options.TemplateURL == "" {
		t, e := template.NewTemplate("str://"+vagrant.VagrantFile, opts)
		if e != nil {
			err = e
			return
		}
		templ = t
	} else {

		// For compatiblity with old code, append a file:// if the
		// value is just a path
		if strings.Index(options.TemplateURL, "://") == -1 {

			p, e := filepath.Abs(options.TemplateURL)
			if e != nil {
				err = e
				return
			}
			options.TemplateURL = "file://localhost" + p
		}

		t, e := template.NewTemplate(options.TemplateURL, opts)
		if e != nil {
			err = e
			return
		}
		templ = t
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: vagrant.NewVagrantPlugin(options.Dir, templ),
	}
	return
}
