package callable

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/callable/backend"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// none is used to determine if the user has set the bool flag value. this allows the
	// use of pipe to a prompt like {{ $foo = flag "flag" "bool" "message" | prompt "foo?" "bool" }}
	none = -1
)

// Options has optional settings for a call.  It can be in a CLI (where Parameters are implemented by flags) or
// programmatic, where Parameters are implemented as maps that can be set in a golang program.
type Options struct {

	// ShowAllWarnings will print all warnings to the console.
	ShowAllWarnings bool

	// Output is the writer used by the backend to write the result.  Typically it's stdout
	OutputFunc func() io.Writer

	// ErrOutputFunc returns a writer for writing errors. Defaults to stderr if not specified
	ErrOutputFunc func() io.Writer

	// Prompter simply prompts the user in some way to retrieve a value of interest.  In the case of CLI,
	// this is would just ask the user and get value from stdin (see PrompterFromReader())
	Prompter func(prompt, ftype string, acceptDefaults bool, optional ...interface{}) (interface{}, error)

	// TemplateOptions has options for processing template
	TemplateOptions template.Options
}

// Clone returns another copy, having defined the parameters
func (c *Callable) Clone(parameters backend.Parameters) (*Callable, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	cc := NewCallable(c.scope, c.src, parameters, c.Options)
	err := cc.DefineParameters()
	return cc, err
}

// Callable is a template that defines parameters and can be executed against
// a supported backend.  The parameters are not thread safe.
type Callable struct {
	backend.Parameters // has methods of Parameters

	Options Options

	*template.Template // has methods of Template

	test           *bool
	printOnly      *bool
	acceptDefaults *bool
	src            string
	exec           bool

	run    func(context.Context, string, backend.Parameters, []string) error
	script string
	scope  scope.Scope

	lock sync.RWMutex
}

// NewCallable creates a callable
func NewCallable(scope scope.Scope, src string, parameters backend.Parameters, options Options) *Callable {
	// Note that because Callable embeds Parameters and implements the methods in Parameters, a Callable
	// can be nested inside another callable..
	return &Callable{
		scope:      scope,
		src:        src,
		Parameters: parameters,
		Options:    options,
	}
}

// name, type, description of the flag, and a default value, which can be nil
// the returned value if the nil value
func (c *Callable) defineParameter(name, ftype, desc string, def interface{}) (interface{}, error) {

	parameters := c.Parameters
	if parameters == nil {
		return nil, nil
	}

	switch ftype {

	case "string":
		defaultValue := ""
		if def != nil {
			if v, ok := def.(string); ok {
				defaultValue = v
			} else {
				return nil, fmt.Errorf("default value not a string: %s", name)
			}
		}
		parameters.String(name, defaultValue, desc)
		return defaultValue, nil

	case "int":
		defaultValue := 0 // TODO - encode a nil with a special value?
		if def != nil {
			if v, ok := def.(int); ok {
				defaultValue = v
			} else {
				return nil, fmt.Errorf("default value not an int: %s", name)
			}
		}
		parameters.Int(name, defaultValue, desc)
		return defaultValue, nil

	case "float":
		defaultValue := float64(0.)
		if def != nil {
			if v, ok := def.(float64); ok {
				defaultValue = v
			} else {
				return nil, fmt.Errorf("default value not a float64: %s", name)
			}
		}
		parameters.Float64(name, defaultValue, desc)
		return defaultValue, nil

	case "ip":
		var defaultValue net.IP
		if def != nil {
			if v, ok := def.(net.IP); ok {
				defaultValue = v
			} else {
				return nil, fmt.Errorf("default value not an ip: %s", name)
			}
		}
		parameters.IP(name, defaultValue, desc)
		return defaultValue, nil

	case "duration":
		var defaultValue time.Duration
		if def != nil {
			if v, ok := def.(time.Duration); ok {
				defaultValue = v
			} else {
				return nil, fmt.Errorf("default value not a duration: %s", name)
			}
		}
		parameters.Duration(name, defaultValue, desc)
		return defaultValue, nil

	case "bool":
		// bool is special in that we want to handle the case of nil --> the flag is not specified
		// so that we can pipe to a prompt if necessary.  pflag does not have the notion of unset
		// flags, so we'd have to hack around it by introducing a string flag if the default is not
		// specified in the template function to define the flag.
		if def != nil {
			// When the default is specified, we cannot use a pipe to prompt.
			// So here just create a bool flag
			defaultValue := false
			switch v := def.(type) {
			case bool:
				defaultValue = v
			case string:
				b, err := strconv.ParseBool(v)
				if err != nil {
					return nil, err
				}
				defaultValue = b
			}
			parameters.Bool(name, defaultValue, desc)
			return defaultValue, nil
		}
		// At definition time, there is no default value, so we use string
		// to model three states: true, false, none
		parameters.Int(name, none, desc)
		return none, nil
	}
	return nil, fmt.Errorf("unknown type %v", ftype)
}

// name, type, description, and default value that can be nil
func (c *Callable) getParameter(name, ftype, desc string, def interface{}) (interface{}, error) {

	parameters := c.Parameters
	if parameters == nil {
		return nil, nil
	}

	switch ftype {

	case "string":
		return parameters.GetString(name)

	case "int":
		return parameters.GetInt(name)

	case "float":
		return parameters.GetFloat64(name)

	case "ip":
		return parameters.GetIP(name)

	case "duration":
		return parameters.GetDuration(name)

	case "bool":
		if def == nil {
			// If default v is not specified, then we assume the flag was defined
			// using a string to handle the none case
			v, err := parameters.GetInt(name)
			if err != nil {
				return none, err
			}
			if v == none {
				return none, nil //
			}
			return v > 0, nil
		}
		return parameters.GetBool(name)
	}

	return nil, nil
}

// Missing returns true if the value v is missing of the type t
func Missing(t string, v interface{}) bool {
	if v == nil {
		return true
	}
	switch t {
	case "string":
		return v.(string) == ""
	case "int":
		return v.(int) == 0
	case "float":
		return v.(float64) == 0.
	case "bool":
		return v == none
	}
	return true
}

func parseBool(text string) (bool, error) {
	if b, err := strconv.ParseBool(text); err == nil {
		return b, nil
	}
	switch text {
	case "y", "Y", "yes", "ok", "OK":
		return true, nil
	case "n", "N", "no", "nope":
		return false, nil
	}
	v, err := strconv.Atoi(text)
	return v > 0, err
}

// PrompterFromReader returns a Prompter that gets a value from a io.Reader (which is usually stdin in the case of CLI)
func PrompterFromReader(in io.Reader) func(prompt, ftype string, acceptDefaults bool, optional ...interface{}) (interface{}, error) {
	return func(prompt, ftype string, acceptDefaults bool, optional ...interface{}) (interface{}, error) {
		return doPrompt(in, prompt, ftype, acceptDefaults, optional...)
	}
}

// doPrompt handles prompting the user using the given prompt message, type string and optional values.
func doPrompt(in io.Reader, prompt, ftype string, acceptDefaults bool, optional ...interface{}) (interface{}, error) {
	def, label := "", ""
	if len(optional) > 0 {
		def = fmt.Sprintf("%v", optional[0])
		if def != "" {
			label = fmt.Sprintf("[%s]", def)
		}
	}

	text := def
	if !acceptDefaults {

		// TODO(chungers) - something fancier so we can support reading of passwords without echoing to screen
		input := bufio.NewReader(in)
		fmt.Fprintf(os.Stderr, "%s %s: ", prompt, label)
		str, _ := input.ReadString('\n')
		text = strings.Trim(str, " \t\n")

		if len(text) == 0 {
			text = def
		}
	}

	switch ftype {
	case "string":
		return text, nil
	case "float":
		return strconv.ParseFloat(text, 64)
	case "int":
		if i, err := strconv.Atoi(text); err == nil {
			return i, nil
		}
		// special case -- int can be used to implement a bool if a default is not provided
		// so we need to handle parsing int from text for purpose of determining a bool
		b, err := parseBool(text)
		if err != nil {
			return b, err
		}
		if b {
			return 1, nil
		}
		return 0, nil
	case "bool":
		if b, err := parseBool(text); err == nil {
			return b, nil
		}
		return nil, fmt.Errorf("cannot parse input for boolean: %v", text)
	}
	return nil, nil // don't err, just pass through
}

func (c *Callable) defineOrGetParameter(n, ftype, desc string, optional ...interface{}) (interface{}, error) {

	if ftype == "" {
		return nil, fmt.Errorf("missing type for variable %v", n)
	}
	var defaultValue interface{}
	if len(optional) > 0 {
		defaultValue = optional[0]
	}
	if c.exec {
		return c.getParameter(n, ftype, desc, defaultValue)
	}

	// Defining a flag never returns a printable value that can mess up the template rendering
	return c.defineParameter(n, ftype, desc, defaultValue)
}

func (c *Callable) defineOrGetParameterList(n, ftype, desc string, optional ...interface{}) ([]string, error) {

	parameters := c.Parameters
	if parameters == nil {
		return nil, nil
	}

	if ftype == "" {
		return nil, fmt.Errorf("missing type for variable %v", n)
	}
	if ftype != "string" {
		return nil, fmt.Errorf("list flag only support string %v", n)
	}
	var defaultValue interface{}
	if len(optional) > 0 {
		defaultValue = optional[0]
	}
	if c.exec {
		d, err := parameters.GetString(n)
		return strings.Split(d, ","), err
	}

	// Defining a flag never returns a printable value that can mess up the template rendering
	d, err := c.defineParameter(n, ftype, desc, defaultValue)
	dl := strings.Split(d.(string), ",")
	return dl, err
}

// Funcs returns the template functions
func (c *Callable) Funcs() []template.Function {
	return []template.Function{
		{
			Name: "depend",
			Func: types.NewDepend,
		},
		{
			Name: "flag",
			Func: c.defineOrGetParameter,
		},
		{
			Name: "param",
			Func: c.defineOrGetParameter,
		},
		{
			Name: "listflag",
			Func: c.defineOrGetParameterList,
		},
		{
			Name: "listparam",
			Func: c.defineOrGetParameterList,
		},
		{
			Name: "fetch",
			Func: func(p string, opt ...interface{}) (string, error) {
				// Overrides the base 'file' to account for the fact that
				// some variables in the flag building phase are not set and would
				// cause errors.  In general, use include for loading files whose
				// paths are computed from some flags.  Use 'source' for including
				// sibling templates that also include other flag definitions.
				if c.exec {
					content, err := c.Template.Fetch(p, opt...)
					if err == nil {
						return content, nil
					}
				}
				return "", nil
			},
		},
		{
			Name: "include",
			Func: func(p string, opt ...interface{}) (string, error) {
				// Overrides the base 'include' to account for the fact that
				// some variables in the flag building phase are not set and would
				// cause errors.  In general, use include for loading files whose
				// paths are computed from some flags.  Use 'source' for including
				// sibling templates that also include other flag definitions.
				if c.exec {
					content, err := c.Template.Include(p, opt...)
					if err == nil {
						return content, nil
					}
				}
				return "{}", nil
			},
		},
		{
			Name: "file",
			Func: func(p string, content interface{}) (string, error) {
				if c.exec {
					var buff []byte
					switch content := content.(type) {
					case []byte:
						buff = content
					case string:
						buff = []byte(content)
					default:
						buff = []byte(fmt.Sprintf("%v", content))
					}
					return "", ioutil.WriteFile(p, buff, 0644)
				}
				return "", nil
			},
		},
		{
			Name: "cond",
			Func: func(b interface{}, optional ...interface{}) func() (bool, interface{}) {

				// Technique here is to capture the value in the pipeline from the last stage,
				// store it, and return a function that will evaluate to the boolean value of
				// the first argument.  By doing so we captured the output of the previous stage
				// and allow the next stage to determine if it needs to continue.  If the next
				// stage chooses to not continue, then it can access the value from the stage
				// before the cond.
				//
				// Example  {{ $x := flag `foo` `string` `foo flag` | cond $y | prompt `foo?` `string` }}
				// So if $y evaluates to false, then prompt will not execute.

				var capture interface{}
				if len(optional) > 0 {
					capture = optional[0]
				}

				return func() (bool, interface{}) {

					switch b := b.(type) {
					case bool:
						return b, capture
					case string:
						p, err := parseBool(b)
						if err != nil {
							return false, capture
						}
						return p, capture
					default:
						return b != nil, capture
					}
				}
			},
		},
		{
			Name: "prompt",
			Func: func(prompt, ftype string, optional ...interface{}) (interface{}, error) {

				if ftype == "" {
					return nil, fmt.Errorf("missing type for variable prompted")
				}
				if !c.exec {
					return "", nil
				}

				if len(optional) > 0 {

					end := optional[len(optional)-1]

					// if the last argument is actually a function that is generated
					// by the 'cond' function
					if cond, is := end.(func() (bool, interface{})); is {
						ok, last := cond()
						if !ok {
							return last, nil
						}
						// if the condition evaluates to true, then we'd continue
						// so the trailing arg must look like the cond was not
						// inserted before this -- hence using the value from
						// stage before the cond as the end
						end = last
					}

					// The last value in the optional var args is the value from the previous
					// pipeline.
					if !Missing(ftype, end) {
						return end, nil
					}

				}

				return c.Options.Prompter(prompt, ftype, *c.acceptDefaults, optional...)
			},
		},
		{
			Name: "listprompt",
			Func: func(prompt, ftype string, optional ...interface{}) ([]string, error) {

				if ftype == "" {
					return nil, fmt.Errorf("missing type for variable prompted")
				}
				if ftype == "" {
					return nil, fmt.Errorf("listprompt only support string")
				}
				var pl []string
				if !c.exec {
					return pl, nil
				}

				if len(optional) > 0 {
					end := optional[len(optional)-1]
					if cond, is := end.(func() (bool, interface{})); is {
						ok, last := cond()
						if !ok {
							return pl, nil
						}
						// if the condition evaluates to true, then we'd continue
						// so the trailing arg must look like the cond was not
						// inserted before this -- hence using the value from
						// stage before the cond as the end
						end = last
					}

					// The last value in the optional var args is the value from the previous
					// pipeline.
					if len(end.([]string)) > 1 {
						return end.([]string), nil
					}

				}
				p, err := c.Options.Prompter(prompt, ftype, *c.acceptDefaults, optional...)
				pl = strings.Split(p.(string), ",")
				return pl, err
			},
		},
	}
}

func (c *Callable) getTemplate() (*template.Template, error) {
	if c.Template == nil {
		t, err := c.scope.TemplateEngine(c.src, c.Options.TemplateOptions)
		if err != nil {
			return nil, err
		}
		c.Template = t
	}
	return c.Template, nil
}

// DefineParameters defines the parameters as specified in the template.
func (c *Callable) DefineParameters() (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.Parameters == nil {
		return fmt.Errorf("not initialized")
	}

	c.test = c.Bool("test", false, "True to do a trial run")
	c.printOnly = c.Bool("print-only", false, "True to print the rendered input")
	c.acceptDefaults = c.Bool("accept-defaults", false, "True to accept defaults of prompts and flags")

	t, err := c.getTemplate()
	if err != nil {
		return
	}

	t.SetOptions(c.Options.TemplateOptions)
	_, err = t.Render(c)
	if err != nil && c.Options.ShowAllWarnings {
		log.Warn("Error rendering playbook while defining params", "err", err)
	}

	// add the backend-defined flags. These are flags that are
	// applied as the backend is chosen.  The delimiter here is =% %=
	opt := c.Options.TemplateOptions
	opt.DelimLeft = "=%"
	opt.DelimRight = "%="
	// Determine the backends
	t.SetOptions(opt)
	added := []string{}
	backend.DefineSharedParameters(
		func(funcName string, defineParams backend.DefineParamsFunc) {
			t.AddFunc(funcName,
				func(opt ...interface{}) error {
					if defineParams == nil {
						return nil
					}

					defineParams(c.Parameters)
					return nil
				})
			added = append(added, funcName)
		})
	_, err = t.Render(c)
	// clean up after we rendered...  remove the functions named
	// after the backends.  Later on we will rebind the actual backends
	t.RemoveFunc(added...)

	return
}

// Execute runs the command
func (c *Callable) Execute(ctx context.Context, args []string, out io.Writer) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.Parameters == nil {
		return fmt.Errorf("not initialized")
	}

	c.exec = true

	var t *template.Template

	t, err = c.getTemplate()
	if err != nil {
		return
	}

	opt := c.Options.TemplateOptions

	if c.Options.ErrOutputFunc != nil {
		opt.Stderr = c.Options.ErrOutputFunc
	} else {
		opt.Stderr = func() io.Writer { return os.Stderr }
	}

	// Process the input, render the template
	t.SetOptions(opt)

	script, err := t.Render(c)
	if err != nil {
		return err
	}
	c.script = script
	if *c.printOnly {
		fmt.Print(c.script)
		return nil
	}

	log.Debug("running", "script", script)

	opt = c.Options.TemplateOptions
	opt.DelimLeft = "=%"
	opt.DelimRight = "%="
	// Determine the backends
	t.SetOptions(opt)

	if err := c.loadBackends(t); err != nil {
		return err
	}

	if c.run != nil {

		switch {
		case out != nil:
			ctx = backend.SetWriter(ctx, out)
		case c.Options.OutputFunc != nil:
			ctx = backend.SetWriter(ctx, c.Options.OutputFunc())
		}
		return c.run(ctx, script, c, args)
	}
	return nil
}
