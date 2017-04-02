package local

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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
	cmd    *cobra.Command
	src    string
	input  io.Reader
	exec   bool
	run    func(string) error
	script string
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

// returns true if the value v is missing of the type t
func missing(t string, v interface{}) bool {
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

func (c *Context) prompt(prompt, ftype string) (interface{}, error) {
	input := bufio.NewReader(c.input)
	fmt.Fprintf(os.Stderr, "%s ", prompt)
	text, _ := input.ReadString('\n')
	text = strings.Trim(text, " \t\n")
	switch ftype {
	case "string":
		return text, nil
	case "int":
		return strconv.Atoi(text)
	case "float":
		return strconv.ParseFloat(text, 64)
	case "bool":
		if b, err := strconv.ParseBool(text); err == nil {
			return b, nil
		}
		switch text {
		case "y", "Y", "yes", "ok", "OK":
			return true, nil
		}
		if v, err := strconv.Atoi(text); err == nil {
			return v > 0, nil
		}
		return nil, fmt.Errorf("cannot parse input for boolean: %v", text)
	}
	return nil, nil
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
			Name: "prompt",
			Func: func(prompt, ftype string, optional ...interface{}) (interface{}, error) {

				if ftype == "" {
					return nil, fmt.Errorf("missing type for variable prompted")
				}
				if !c.exec {
					return "", nil
				}
				if len(optional) > 0 && !missing(ftype, optional[0]) {
					return optional[0], nil
				}
				return c.prompt(prompt, ftype)
			},
		},
	}
}

// loadBackend determines the backend to use for executing the rendered template text (e.g. run in shell).
// During this phase, the template delimiters are changed to =% %= so put this in the comment {{/* */}}
func (c *Context) loadBackend() error {
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

				return exec.Command(cmd).
					InheritEnvs(true).StartWithStreams(

					exec.Do(exec.SendInput(
						func(stdin io.WriteCloser) error {
							_, err := stdin.Write([]byte(script))
							return err
						})).Then(
						exec.RedirectStdout(os.Stdout)).Then(
						exec.RedirectStderr(os.Stderr),
					).Done(),
				)
			}
			return ""
		})

	_, err = t.Render(c)
	return err
}

// buildFlags from parsing the body which is a template
func (c *Context) buildFlags() error {
	t, err := template.NewTemplate(c.src, template.Options{})
	if err != nil {
		return err
	}
	_, err = t.Render(c)
	return err
}

func (c *Context) execute() error {
	t, err := template.NewTemplate(c.src, template.Options{
		Stderr: func() io.Writer { return os.Stderr },
	})
	if err != nil {
		return err
	}
	c.exec = true
	script, err := t.Render(c)
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
