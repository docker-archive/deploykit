package exec

import (
	"io"
	"os"
	"os/exec"
	"strings"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/template"
)

var log = logutil.New("module", "util/exec")

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

// String returns the interpolated version of the command
func (c Command) String(args ...string) (string, error) {
	p, err := c.builder().generate(args...)
	if err == nil {
		return strings.Join(p, " "), nil
	}
	return string(c), err
}

// WithOptions adds the template options
func (c Command) WithOptions(options template.Options) *Builder {
	return c.builder().WithOptions(options)
}

// WithFunc adds a function that can be used in the template
func (c Command) WithFunc(name string, function interface{}) *Builder {
	return c.builder().WithFunc(name, function)
}

// WithContext sets the context for the template
func (c Command) WithContext(context interface{}) *Builder {
	return c.builder().WithContext(context)
}

// InheritEnvs determines whether the process should inherit the envs of the parent
func (c Command) InheritEnvs(v bool) *Builder {
	return c.builder().InheritEnvs(v)
}

// NewCommand creates an instance of the command builder to allow detailed configuration
func NewCommand(s string) *Builder {
	return Command(s).builder()
}

// Builder collects options until it's run
type Builder struct {
	command     Command
	options     template.Options
	inheritEnvs bool
	envs        []string
	funcs       map[string]interface{}
	context     interface{}
	rendered    string // rendered command string
	cmd         *exec.Cmd
}

func (c Command) builder() *Builder {
	return &Builder{
		command: c,
		funcs:   map[string]interface{}{},
	}
}

// InheritEnvs determines whether the process should inherit the envs of the parent
func (b *Builder) InheritEnvs(v bool) *Builder {
	b.inheritEnvs = v
	return b
}

// WithEnvs adds environment variables for the exec, in format of key=value
func (b *Builder) WithEnvs(kv ...string) *Builder {
	b.envs = append(b.envs, kv...)
	return b
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

// Step is something you do with the processes streams
type Step func(stdin io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser) error

// Thenable is a fluent builder for chaining tasks
type Thenable struct {
	steps []Step
}

// Do creates a thenable
func Do(f Step) *Thenable {
	return &Thenable{
		steps: []Step{f},
	}
}

// Then adds another step
func (t *Thenable) Then(then Step) *Thenable {
	t.steps = append(t.steps, then)
	return t
}

// Done returns the final function
func (t *Thenable) Done() Step {
	steps := t.steps
	return func(stdin io.WriteCloser, stdout, stderr io.ReadCloser) error {
		for _, step := range steps {
			if err := step(stdin, stdout, stderr); err != nil {
				return err
			}
		}
		return nil
	}
}

// SendInput is a convenience function for writing to the exec process's stdin. When the function completes, the
// stdin is closed.
func SendInput(f func(io.WriteCloser) error) Step {
	return func(stdin io.WriteCloser, stdout, stderr io.ReadCloser) error {
		defer stdin.Close()
		return f(stdin)
	}
}

// MergeOutput combines the stdout and stderr into the given stream
func MergeOutput(out io.Writer) Step {
	return func(stdin io.WriteCloser, stdout, stderr io.ReadCloser) error {
		_, err := io.Copy(out, io.MultiReader(stdout, stderr))
		return err
	}
}

// StartWithStreams starts the the process and then calls the function which allows
// the streams to be wired.  Calling the provided function blocks.
func (b *Builder) StartWithStreams(f Step,
	args ...string) error {

	_, err := b.exec(args...)
	if err != nil {
		return err
	}

	pOut, err := b.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	pErr, err := b.cmd.StderrPipe()
	if err != nil {
		return err
	}
	pIn, err := b.cmd.StdinPipe()
	if err != nil {
		return err
	}

	err = b.cmd.Start()
	if err != nil {
		return err
	}

	return f(pIn, pOut, pErr)
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
	log.Debug("exec", "command", command)
	b.cmd = exec.Command(command[0], command[1:]...)
	if b.inheritEnvs {
		b.cmd.Env = append(os.Environ(), b.envs...)
	}

	return b.cmd, nil
}
