package exec

import (
	"os/exec"
	"strings"

	"github.com/docker/infrakit/pkg/template"
	log "github.com/golang/glog"
)

// Command is the template which is rendered before it's executed
type Command string

// Output runs the command until completion and returns the results
func (c Command) Output(args ...string) ([]byte, error) {
	return c.builder().Output(args...)
}

// Start runs the command without blocking
func (c Command) Start(args ...string) error {
	return c.builder().Start(args...)
}

// Run does a Cmd.Run on the command
func (c Command) Run(args ...string) error {
	return c.builder().Run(args...)
}

// WithOptions adds the template options
func (c Command) WithOptions(options template.Options) *Builder {
	b := c.builder()
	b.options = options
	return b
}

// WithFunc adds a function that can be used in the template
func (c Command) WithFunc(name string, function interface{}) *Builder {
	b := c.builder()
	b.funcs[name] = function
	return b
}

// WithContext sets the context for the template
func (c Command) WithContext(context interface{}) *Builder {
	b := c.builder()
	b.context = context
	return b
}

// Builder collects options until it's run
type Builder struct {
	command  Command
	options  template.Options
	funcs    map[string]interface{}
	context  interface{}
	rendered string // rendered command string
	cmd      *exec.Cmd
}

func (c Command) builder() *Builder {
	return &Builder{
		command: c,
		funcs:   map[string]interface{}{},
	}
}

// WithOptions adds the template options
func (b *Builder) WithOptions(options template.Options) *Builder {
	b.options = options
	return b
}

// WithFunc adds a function that can be used in the template
func (b *Builder) WithFunc(name string, function interface{}) *Builder {
	b.funcs[name] = function
	return b
}

// WithContext sets the context of the template
func (b *Builder) WithContext(context interface{}) *Builder {
	b.context = context
	return b
}

// Output runs the command until completion and returns the results
func (b *Builder) Output(args ...string) ([]byte, error) {
	run, err := b.exec(args...)
	if err != nil {
		return nil, err
	}
	return run.Output()
}

// Start runs the command without blocking
func (b *Builder) Start(args ...string) error {
	run, err := b.exec(args...)
	if err != nil {
		return err
	}
	return run.Start()
}

// Run does a Cmd.Run on the command
func (b *Builder) Run(args ...string) error {
	run, err := b.exec(args...)
	if err != nil {
		return err
	}
	return run.Run()
}

func (b *Builder) generate(args ...string) ([]string, error) {
	ct, err := template.NewTemplate("str://"+string(b.command), template.Options{})
	if err != nil {
		return nil, err
	}
	for k, v := range b.funcs {
		ct.AddFunc(k, v)
	}
	ct.AddFunc("arg", func(i int) interface{} {
		return args[i-1] // starts at 1
	})
	ct.AddFunc("argv", func() interface{} {
		return args
	})
	cmd, err := ct.Render(b.context)
	if err != nil {
		return nil, err
	}

	cmd = strings.Replace(cmd, "\\\n", "", -1)
	command := []string{}
	for _, s := range strings.Split(cmd, " ") {
		s = strings.Trim(s, " \t\n")
		if len(s) > 0 {
			command = append(command, s)
		}
	}
	return command, nil
}
func (b *Builder) exec(args ...string) (*exec.Cmd, error) {
	command, err := b.generate(args...)
	if err != nil {
		return nil, err
	}
	log.V(50).Infoln("exec:", command)
	b.cmd = exec.Command(command[0], command[1:]...)
	return b.cmd, nil
}
