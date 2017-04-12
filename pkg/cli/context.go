package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/util/exec"
	"github.com/spf13/cobra"
)

const (
	// none is used to determine if the user has set the bool flag value. this allows the
	// use of pipe to a prompt like {{ $foo = flag "flag" "bool" "message" | prompt "foo?" "bool" }}
	none = -1
)

// Context is the context for the running module
type Context struct {
	cmd     *cobra.Command
	src     string
	input   io.Reader
	exec    bool
	run     func(string) error
	script  string
	plugins func() discovery.Plugins
}

// NewContext creates a context
func NewContext(plugins func() discovery.Plugins, cmd *cobra.Command, src string, input io.Reader) *Context {
	return &Context{
		plugins: plugins,
		cmd:     cmd,
		src:     src,
		input:   input,
	}
}

// name, type, description of the flag, and a default value, which can be nil
// the returned value if the nil value
func (c *Context) defineFlag(name, ftype, desc string, def interface{}) (interface{}, error) {
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
		c.cmd.Flags().String(name, defaultValue, desc)
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
		c.cmd.Flags().Int(name, defaultValue, desc)
		return defaultValue, nil

	case "float":
		defaultValue := 0.
		if def != nil {
			if v, ok := def.(float64); ok {
				defaultValue = v
			} else {
				return nil, fmt.Errorf("default value not a float64: %s", name)
			}
		}
		c.cmd.Flags().Float64(name, defaultValue, desc)
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
			c.cmd.Flags().Bool(name, defaultValue, desc)
			return defaultValue, nil
		}
		// At definition time, there is no default value, so we use string
		// to model three states: true, false, none
		c.cmd.Flags().Int(name, none, desc)
		return none, nil
	}
	return nil, fmt.Errorf("unknown type %v", ftype)
}

// name, type, description, and default value that can be nil
func (c *Context) getFromFlag(name, ftype, desc string, def interface{}) (interface{}, error) {

	switch ftype {

	case "string":
		return c.cmd.Flags().GetString(name)

	case "int":
		return c.cmd.Flags().GetInt(name)

	case "float":
		return c.cmd.Flags().GetFloat64(name)

	case "bool":
		if def == nil {
			// If default v is not specified, then we assume the flag was defined
			// using a string to handle the none case
			v, err := c.cmd.Flags().GetInt(name)
			if err != nil {
				return none, err
			}
			if v == none {
				return none, nil //
			}
			return v > 0, nil
		}
		return c.cmd.Flags().GetBool(name)
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

// Prompt handles prompting the user using the given prompt message, type string and optional values.
func Prompt(in io.Reader, prompt, ftype string, optional ...interface{}) (interface{}, error) {
	def, label := "", ""
	if len(optional) > 0 {
		def = fmt.Sprintf("%v", optional[0])
		if def != "" {
			label = fmt.Sprintf("[%s]", def)
		}
	}

	input := bufio.NewReader(in)
	fmt.Fprintf(os.Stderr, "%s %s: ", prompt, label)
	text, _ := input.ReadString('\n')
	text = strings.Trim(text, " \t\n")
	if len(text) == 0 {
		text = def
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

// Funcs returns the template functions
func (c *Context) Funcs() []template.Function {
	return []template.Function{
		{
			Name: "flag",
			Func: func(n, ftype, desc string, optional ...interface{}) (interface{}, error) {
				if ftype == "" {
					return nil, fmt.Errorf("missing type for variable %v", n)
				}
				var defaultValue interface{}
				if len(optional) > 0 {
					defaultValue = optional[0]
				}
				if c.exec {
					return c.getFromFlag(n, ftype, desc, defaultValue)
				}

				// Defining a flag never returns a printable value that can mess up the template rendering
				return c.defineFlag(n, ftype, desc, defaultValue)
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
				return Prompt(c.input, prompt, ftype, optional...)
			},
		},
	}
}

// loadBackend determines the backend to use for executing the rendered template text (e.g. run in shell).
// During this phase, the template delimiters are changed to =% %= so put this in the comment {{/* */}}
func (c *Context) loadBackends() error {
	t, err := template.NewTemplate(c.src, template.Options{
		DelimLeft:  "=%",
		DelimRight: "%=",
	})
	if err != nil {
		return err
	}
	t.AddFunc("print",
		func() string {
			c.run = func(script string) error {
				fmt.Println(script)
				return nil
			}
			return ""
		})
	t.AddFunc("sh",
		func(opts ...string) string {
			c.run = func(script string) error {

				cmd := strings.Join(append([]string{"/bin/sh"}, opts...), " ")
				log.Debug("sh", "cmd", cmd)

				run := exec.Command(cmd)
				run.InheritEnvs(true).StartWithHandlers(
					func(stdin io.Writer) error {
						_, err := stdin.Write([]byte(script))
						return err
					},
					func(stdout io.Reader) error {
						_, err := io.Copy(os.Stdout, stdout)
						return err
					},
					func(stderr io.Reader) error {
						_, err := io.Copy(os.Stderr, stderr)
						return err
					},
				)
				return run.Wait()
			}
			return ""
		})

	_, err = t.Render(c)
	return err
}

// BuildFlags from parsing the body which is a template
func (c *Context) BuildFlags() error {
	t, err := template.NewTemplate(c.src, template.Options{})
	if err != nil {
		return err
	}

	_, err = ConfigureTemplate(t, c.plugins).Render(c)
	return err
}

// Execute runs the command
func (c *Context) Execute() error {

	if err := c.loadBackends(); err != nil {
		return err
	}

	t, err := template.NewTemplate(c.src, template.Options{
		Stderr: func() io.Writer { return os.Stderr },
	})
	if err != nil {
		return err
	}

	c.exec = true
	script, err := ConfigureTemplate(t, c.plugins).Render(c)
	if err != nil {
		return err
	}
	c.script = script
	log.Debug("running", "script", script)
	if c.run != nil {
		return c.run(script)
	}
	return nil
}
