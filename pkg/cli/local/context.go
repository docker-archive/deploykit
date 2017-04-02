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

// Context is the context for the running module
type Context struct {
	cmd    *cobra.Command
	src    string
	input  io.Reader
	exec   bool
	run    func(string) error
	script string
}

func (c *Context) defineFlag(name, ftype, desc string, v interface{}) (interface{}, error) {

	switch ftype {
	case "string":
		defaultValue := ""
		if v, ok := v.(string); ok {
			defaultValue = v
		}
		c.cmd.Flags().String(name, defaultValue, desc)
		return defaultValue, nil

	case "int":
		defaultValue := 0
		if v, ok := v.(int); ok {
			defaultValue = v
		}
		c.cmd.Flags().Int(name, defaultValue, desc)
		return defaultValue, nil

	case "bool":
		defaultValue := false
		if v, ok := v.(bool); ok {
			defaultValue = v
		}
		c.cmd.Flags().Bool(name, defaultValue, desc)
		return defaultValue, nil

	case "float":
		defaultValue := 0.
		if v, ok := v.(float64); ok {
			defaultValue = v
		}
		c.cmd.Flags().Float64(name, defaultValue, desc)
		return defaultValue, nil
	}
	return nil, nil
}

func (c *Context) getFromFlag(name, ftype, desc string, v interface{}) (interface{}, error) {

	switch ftype {
	case "string":
		return c.cmd.Flags().GetString(name)

	case "int":
		return c.cmd.Flags().GetInt(name)

	case "bool":
		return c.cmd.Flags().GetBool(name)

	case "float":
		return c.cmd.Flags().GetFloat64(name)
	}

	return nil, nil
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
	case "bool":
		return strconv.ParseBool(text)
	case "float":
		return strconv.ParseFloat(text, 64)
	}
	return nil, nil
}

// returns true if the value v is a zero of the type t
func zero(t string, v interface{}) bool {
	if v == nil {
		return true
	}

	switch t {
	case "string":
		return v.(string) == ""
	case "int":
		return v.(int) == 0
	case "bool":
		return v.(bool) == false
	case "float":
		return v.(float64) == 0.
	}

	return true
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
				var v interface{}
				if len(optional) > 0 {
					v = optional[0]
				}
				if c.exec {
					return c.getFromFlag(n, ftype, desc, v)
				}
				return c.defineFlag(n, ftype, desc, v)
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
				if len(optional) > 0 && !zero(ftype, optional[0]) {
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
