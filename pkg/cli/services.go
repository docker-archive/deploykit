package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/ghodss/yaml"
	"github.com/spf13/pflag"
)

// Services is a common set of utiities that are available to each command.
// For example, plugin lookup, metadata lookup, template engine
type Services struct {

	// Scope is the scope this runs in
	Scope scope.Scope

	// ProcessTemplateFlags are common flags associated with the base services.  They should be added to subcommands
	// if the subcommands make use of the services provided here.
	ProcessTemplateFlags *pflag.FlagSet

	// ProcessTemplate is the function that processes the template at url and returns view or error.
	ProcessTemplate ProcessTemplateFunc

	// ToJSON converts the input buffer to json format
	ToJSON ToJSONFunc

	// FromJSON converts json formatted input to output buffer
	FromJSON FromJSONFunc

	// OutputFlags are flags that control output format
	OutputFlags *pflag.FlagSet

	// Output is the function that does output
	Output OutputFunc
}

// NewServices creates an instance of common services for all commands
func NewServices(scope scope.Scope) *Services {
	flags, toJSON, fromJSON, processTemplate := templateProcessor(scope)
	outputFlags, outputFunc := Output()
	return &Services{
		Scope:                scope,
		ProcessTemplateFlags: flags,
		ProcessTemplate:      processTemplate,
		ToJSON:               toJSON,
		FromJSON:             fromJSON,
		OutputFlags:          outputFlags,
		Output:               outputFunc,
	}
}

// ProcessTemplateFunc is the function that processes the template at url and returns view or error.
type ProcessTemplateFunc func(url string, ctx ...interface{}) (rendered string, err error)

// ToJSONFunc converts the input buffer to json format
type ToJSONFunc func(in []byte) (json []byte, err error)

// FromJSONFunc converts json formatted input to output buffer
type FromJSONFunc func(json []byte) (out []byte, err error)

// ReadFromStdinOrURL reads input given from the argument.  If this arg is URL then the content
// is fetched and rendered as a template.  If the arg is '-', it reads from stdin.
func (s *Services) ReadFromStdinOrURL(arg string) (rendered string, err error) {
	return s.ReadFromStdinIfElse(
		func() bool { return arg == "-" },
		func() (string, error) { return s.ProcessTemplate(arg) },
		s.ToJSON,
	)
}

// ReadFromStdinIfElse checks condition and reads from stdin if true; otherwise it executes other.
func (s *Services) ReadFromStdinIfElse(condition func() bool,
	otherwise func() (string, error),
	toJSON ToJSONFunc) (rendered string, err error) {

	if condition() {
		buff, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		json, err := toJSON(buff)
		log.Debug("stdin", "buffer", string(json))
		return string(json), err
	}
	rendered, err = otherwise()
	if err != nil {
		return
	}
	var buff []byte
	buff, err = toJSON([]byte(rendered))
	if err != nil {
		return
	}
	return string(buff), nil
}

// templateProcessor returns a flagset and a function for processing template input.
func templateProcessor(scope scope.Scope) (*pflag.FlagSet,
	ToJSONFunc, FromJSONFunc, ProcessTemplateFunc) {

	fs := pflag.NewFlagSet("template", pflag.ExitOnError)

	globals := fs.StringSlice("var", []string{}, "key=value pairs of globally scoped variables")
	yamlDoc := fs.BoolP("yaml", "y", false, "True if input is in yaml format; json is the default")
	dump := fs.BoolP("dump", "x", false, "True to dump to output instead of executing")
	singlePass := fs.BoolP("final", "f", false, "True to render template as the final pass")

	return fs,
		// ToJSONFunc
		func(in []byte) (json []byte, err error) {

			defer func() {

				if *dump {
					fmt.Println("Raw:")
					fmt.Println(string(in))
					fmt.Println("Converted")
					fmt.Println(string(json))
					os.Exit(0) // special for debugging
				}
			}()

			if *yamlDoc {
				json, err = yaml.YAMLToJSON(in)
				return
			}
			json = in
			return

		},
		// FromJSONFunc
		func(json []byte) (out []byte, err error) {

			defer func() {

				if *dump {
					fmt.Println("Raw:")
					fmt.Println(string(json))
					fmt.Println("Converted")
					fmt.Println(string(out))
					os.Exit(0) // special for debugging
				}
			}()

			if *yamlDoc {
				out, err = yaml.JSONToYAML(json)
				return
			}
			out = json
			return

		},
		// ProcessTemplateFunc
		func(url string, ctx ...interface{}) (view string, err error) {

			if !strings.Contains(url, "://") {
				p := url
				if dir, err := os.Getwd(); err == nil {
					p = path.Join(dir, url)
				}
				if _, err := os.Stat(p); os.IsNotExist(err) {
					p = url
				}
				url = "file://" + p
			}

			log.Debug("reading template", "url", url)
			engine, err := scope.TemplateEngine(url, template.Options{MultiPass: !*singlePass})
			if err != nil {
				return
			}

			for _, global := range *globals {
				kv := strings.SplitN(global, "=", 2)
				if len(kv) != 2 {
					log.Warn("bad format kv", "input", global)
					continue
				}
				key := strings.TrimSpace(kv[0])
				val := strings.TrimSpace(kv[1])
				if key != "" && val != "" {
					// Attempt to convert to int and bool types so that template operations
					// are not only against strings.
					if intVal, err := strconv.Atoi(val); err == nil {
						engine.Global(key, intVal)
					} else if boolVar, err := strconv.ParseBool(val); err == nil {
						engine.Global(key, boolVar)
					} else {
						engine.Global(key, val)
					}
				}
			}

			contextObject := (interface{})(nil)
			if len(ctx) == 1 {
				contextObject = ctx[0]
			}
			view, err = engine.Render(contextObject)
			if err != nil {
				return
			}

			log.Debug("rendered", "view", view)
			if *dump {
				fmt.Println("Final:")
				fmt.Println(string(view))
				os.Exit(0)
			}
			return
		}
}
