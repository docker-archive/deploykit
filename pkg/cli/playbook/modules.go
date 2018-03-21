package playbook

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/docker/infrakit/pkg/callable"
	"github.com/docker/infrakit/pkg/callable/backend"
	"github.com/docker/infrakit/pkg/cli"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/playbook")

// helpTemplate is for embedding content from README.md in the same directory.
const helpTemplate = `{{with or .Long .Short }}{{. | trim}}

%s
{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}
`

type remote struct {
	modules Modules
	input   io.Reader
	scope   scope.Scope
	options Options
}

// Op is the name of the operation / sub-command
type Op string

// SourceURL is the url of the module
type SourceURL string

// Modules is a mapping of operations and the source url that defines it
type Modules map[Op]SourceURL

// Options contains tuning params for the behavior of the templating engine and callables.
type Options struct {

	// ShowAllWarnings will print all warnings to the console.
	ShowAllWarnings bool

	template.Options `json:",inline" yaml:",inline"`
}

// NewModules returns an implementation of Modules using a file at given URL. The file is in YAML format
func NewModules(scope scope.Scope, modules Modules, input io.Reader,
	options Options) (cli.Modules, error) {

	return &remote{
		modules: modules,
		input:   input,
		scope:   scope,
		options: options,
	}, nil
}

// Decode parses and loads the file that contains the module definitions
func Decode(data []byte, m *Modules) error {
	err := types.AnyBytes(data).Decode(m)
	if err == nil {
		return nil
	}
	buff, err := yaml.YAMLToJSON([]byte(data))
	if err != nil {
		return err
	}
	return types.AnyBytes(buff).Decode(m)
}

// Encode returns the encode bytes of the module
func Encode(m Modules) ([]byte, error) {
	any, err := types.AnyValue(m)
	if err != nil {
		return nil, err
	}
	return yaml.JSONToYAML(any.Bytes())
}

func dir(url SourceURL, options Options) (Modules, error) {
	t, err := template.NewTemplate(string(url), options.Options)
	if err != nil {
		return nil, err
	}

	view, err := t.Render(nil)
	if err != nil {
		return nil, err
	}

	m := Modules{}
	err = Decode([]byte(view), &m)
	if err == nil {
		return m, nil
	}
	m[Op(".")] = url
	return m, err
}

func list(context context.Context, scope scope.Scope, modules Modules, input io.Reader,
	parent *cobra.Command, parentURL *SourceURL, options Options) ([]*cobra.Command, error) {

	found := []*cobra.Command{}

loop:
	for op, moduleURL := range modules {

		cmd := &cobra.Command{
			Use:   string(op),
			Short: string(op),
		}

		var parent *url.URL

		if parentURL != nil {

			// try to resolve to absolute url if it's relative
			if u, err := template.GetURL(string(*parentURL), string(moduleURL)); err == nil {

				parent = u
				moduleURL = SourceURL(u.String())

			} else {
				log.Debug("cannot resolve", "op", op, "url", moduleURL, "parent", parentURL)
				continue loop
			}
		}

		// Documentation -- look for a README.md at the given dir
		readmeURLStr := path.Join(path.Dir(string(moduleURL)), "README.md")
		if parent != nil {

			// Documentation -- look for a README.md at the given dir
			readmeURL := *parent
			readmeURL.Path = path.Join(path.Dir(parent.Path), "README.md")
			readmeURLStr = readmeURL.String()
		}
		if t, err := template.NewTemplate(readmeURLStr, options.Options); err == nil {
			if view, err := t.Render(nil); err == nil {
				cmd.SetHelpTemplate(fmt.Sprintf(helpTemplate, view))
			}
		}

		// if we can parse it as a map, then we have a 'directory'
		mods, err := dir(moduleURL, options)
		if err == nil {

			copy := moduleURL
			subs, err := list(context, scope, mods, input, cmd, &copy, options)
			if err != nil {
				log.Debug("cannot list", "op", op, "url", moduleURL, "err", err)
				continue loop
			}
			for _, sub := range subs {
				cmd.AddCommand(sub)
			}

		} else {

			callable := callable.NewCallable(scope, string(moduleURL),
				callable.ParametersFromFlags(cmd.Flags()),
				callable.Options{
					ShowAllWarnings: options.ShowAllWarnings,
					TemplateOptions: options.Options,
					Prompter:        callable.PrompterFromReader(input),
				})
			err := callable.DefineParameters()
			if err != nil {
				log.Warn("Cannot build flags", "operation", op, "url", moduleURL, "err", err)
				continue loop
			}
			cmd.RunE = func(c *cobra.Command, args []string) error {
				log.Debug("Running", "command", op, "url", moduleURL, "args", args)
				return callable.Execute(context, args, nil)
			}

		}
		found = append(found, cmd)
	}
	return found, nil
}

// List returns a list of commands defined in the modules
func (r *remote) List() ([]*cobra.Command, error) {
	// Because we don't have the parent urls, the urls specified in the modules all must be absolute
	if err := resolved(r.modules); err != nil {
		return nil, err
	}

	ctx := backend.SetWriter(context.Background(), os.Stdout)
	return list(ctx, r.scope, r.modules, r.input, nil, nil, r.options)
}

func resolved(m Modules) error {
	for _, url := range m {
		if !strings.Contains(string(url), "://") {
			return fmt.Errorf("not a full url: %s", url)
		}
	}
	return nil
}
