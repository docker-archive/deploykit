package exec

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/template"
)

var log = logutil.New("module", "util/exec")

// Command returns a fluent builder for running a command where the command string
// can have template functions and arguments
func Command(s string) *Builder {
	return &Builder{
		command: s,
		funcs:   map[string]interface{}{},
		args:    map[interface{}]interface{}{},
	}
}

// Builder collects options until it's run
type Builder struct {
	command     string
	options     template.Options
	inheritEnvs bool
	envs        []string
	funcs       map[string]interface{}
	args        map[interface{}]interface{}
	context     interface{}
	rendered    string // rendered command string
	cmd         *exec.Cmd
}

// WithArg sets the arg key, value pair that can be accessed via the 'arg' function
func (b *Builder) WithArg(key string, value interface{}) *Builder {
	b.args[key] = value
	return b
}

// WithArgs adds the command line args array
func (b *Builder) WithArgs(args ...interface{}) *Builder {
	for i, arg := range args {
		b.args[i+1] = arg
	}
	return b
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
	all := t.steps
	return func(stdin io.WriteCloser, stdout, stderr io.ReadCloser) error {
		for _, next := range all {
			if err := next(stdin, stdout, stderr); err != nil {
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

// RedirectStdout sends stdout to given writer
func RedirectStdout(out io.Writer) Step {
	return func(stdin io.WriteCloser, stdout, stderr io.ReadCloser) error {
		_, err := io.Copy(out, stdout)
		return err
	}
}

// RedirectStderr sends stdout to given writer
func RedirectStderr(out io.Writer) Step {
	return func(stdin io.WriteCloser, stdout, stderr io.ReadCloser) error {
		_, err := io.Copy(out, stderr)
		return err
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
func (b *Builder) StartWithStreams(f Step, args ...interface{}) error {

	if err := b.Prepare(args...); err != nil {
		return err
	}

	run := func() error { return nil }
	if f != nil {
		pIn, err := b.cmd.StdinPipe()
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

		run = func() error {
			return f(pIn, pOut, pErr)
		}
	}

	if err := b.cmd.Start(); err != nil {
		return err
	}

	return run()
}

// Start does a Cmd.Start on the command
func (b *Builder) Start(args ...interface{}) error {
	if err := b.Prepare(args...); err != nil {
		return err
	}
	return b.StartWithStreams(nil, args...)
}

// Wait waits for the command to complete
func (b *Builder) Wait() error {
	return b.cmd.Wait()
}

// Output runs the command until completion and returns the results
func (b *Builder) Output(args ...interface{}) ([]byte, error) {
	if err := b.Prepare(args...); err != nil {
		return nil, err
	}
	return b.cmd.Output()
}

func (b *Builder) generate(args ...interface{}) ([]string, error) {
	// also index the args by index
	for i, v := range args {
		b.args[i+1] = v
	}

	ct, err := template.NewTemplate("str://"+string(b.command), template.Options{})
	if err != nil {
		return nil, err
	}
	for k, v := range b.funcs {
		ct.AddFunc(k, v)
	}
	ct.AddFunc("arg", func(i interface{}) interface{} {
		if i, is := i.(int); is {
			if len(args) > i {
				return args[i-1] // starts at 1
			}
		}
		return b.args[i]
	})
	ct.AddFunc("argv", func() interface{} {
		argv := []string{}
		for _, arg := range args {
			argv = append(argv, fmt.Sprintf("%v", arg))
		}
		return argv
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

// Prepare generates the command based on the input args. This is the step before actual Start or Run
func (b *Builder) Prepare(args ...interface{}) error {
	command, err := b.generate(args...)
	if err != nil {
		return err
	}
	log.Debug("exec", "command", command)
	b.cmd = exec.Command(command[0], command[1:]...)
	if b.inheritEnvs {
		b.cmd.Env = append(os.Environ(), b.envs...)
	}
	return nil
}

// Stdin takes the input from the writer
func (b *Builder) Stdin(f func(w io.Writer) error) error {
	input, err := b.cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer input.Close()
	return f(input)
}

// StdoutTo connects the stdout of this to the next stage
func (b *Builder) StdoutTo(next *Builder) {
	r, w := io.Pipe()
	b.cmd.Stdout = w
	next.cmd.Stdin = r
}

// Stdout sets the stdout
func (b *Builder) Stdout(w io.Writer) {
	b.cmd.Stdout = w
}
