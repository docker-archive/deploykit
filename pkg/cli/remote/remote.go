package remote

import (
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/remote")

// helpTemplate is for embedding content from README.md in the same directory.
const helpTemplate = `{{with or .Long .Short }}{{. | trim}}

%s
{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}
`

type remote struct {
	modules Modules
	input   io.Reader
	plugins func() discovery.Plugins
}

// Op is the name of the operation / sub-command
type Op string

// SourceURL is the url of the module
type SourceURL string

// Modules is a mapping of operations and the source url that defines it
type Modules map[Op]SourceURL

// NewModules returns an implementation of Modules using a file at given URL. The file is in YAML format
func NewModules(plugins func() discovery.Plugins, modules Modules, input io.Reader) (cli.Modules, error) {
	return &remote{
		modules: modules,
		input:   input,
		plugins: plugins,
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

func dir(url SourceURL) (Modules, error) {
	t, err := template.NewTemplate(string(url), template.Options{})
	if err != nil {
		return nil, err
	}

	view, err := t.Render(nil)
	if err != nil {
		return nil, err
	}

	m := Modules{}
	err = Decode([]byte(view), &m)
	return m, err
}

func list(plugins func() discovery.Plugins, modules Modules, input io.Reader,
	parent *cobra.Command, parentURL *SourceURL) ([]*cobra.Command, error) {

	found := []*cobra.Command{}

loop:
	for op, url := range modules {

		cmd := &cobra.Command{
			Use:   string(op),
			Short: string(op),
		}

		// try to resolve to absolute url if it's relative
		if parentURL != nil {
			if u, err := template.GetURL(string(*parentURL), string(url)); err == nil {
				url = SourceURL(u)
			} else {
				log.Warn("cannot resolve", "op", op, "url", url, "parent", parentURL)
				continue loop
			}
		}

		// if we can parse it as a map, then we have a 'directory'
		mods, err := dir(url)
		if err == nil {

			// Documentation -- look for a README.md at the given dir
			readme := path.Join(path.Dir(string(url)), "README.md")
			if t, err := template.NewTemplate(readme, template.Options{}); err == nil {
				if view, err := t.Render(nil); err == nil {
					cmd.SetHelpTemplate(fmt.Sprintf(helpTemplate, view))
				}
			}

			copy := url
			subs, err := list(plugins, mods, input, cmd, &copy)
			if err != nil {
				log.Warn("cannot list", "op", op, "url", url, "err", err)
				continue loop
			}
			for _, sub := range subs {
				cmd.AddCommand(sub)
			}
		} else {
			ctx := cli.NewContext(plugins, cmd, string(url), input)
			cmd.RunE = func(c *cobra.Command, args []string) error {
				log.Debug("Running", "command", op, "url", url, "args", args)
				return ctx.Execute()
			}
			err := ctx.BuildFlags()
			if err != nil {
				log.Warn("cannot build flags", "op", op, "url", url, "err", err)
				continue loop
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
	return list(r.plugins, r.modules, r.input, nil, nil)
}

func resolved(m Modules) error {
	for _, url := range m {
		if !strings.Contains(string(url), "://") {
			return fmt.Errorf("not a full url: %s", url)
		}
	}
	return nil
}
