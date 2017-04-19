package exec

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

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
	dir         string
	envs        []string
	funcs       map[string]interface{}
	args        map[interface{}]interface{}
	context     interface{}
	rendered    string // rendered command string
	cmd         *exec.Cmd
	stdout      io.Writer
	stderr      io.Writer
	stdin       io.Reader
	wg          sync.WaitGroup
}

// WithStdin sets the stdin reader
func (b *Builder) WithStdin(r io.Reader) *Builder {
	b.stdin = r
	return b
}

// WithStdout sets the stdout writer
func (b *Builder) WithStdout(w io.Writer) *Builder {
	b.stdout = w
	return b
}

// WithStderr sets the stderr writer
func (b *Builder) WithStderr(w io.Writer) *Builder {
	b.stdout = w
	return b
}

// WithDir sets the working directory.
func (b *Builder) WithDir(path string) *Builder {
	b.dir = path
	return b
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

var noop = func() error { return nil }

// StartWithHandlers starts the cmd non blocking and calls the given handlers to process input / output
func (b *Builder) StartWithHandlers(stdinFunc func(io.Writer) error,
	stdoutFunc func(io.Reader) error,
	stderrFunc func(io.Reader) error,
	args ...interface{}) error {

	if err := b.Prepare(args...); err != nil {
		return err
	}

	// There's a race between the input/output streams reads and cmd.wait() which
	// will close the pipes even while others are trying to read.
	// So we need to ensure that all the input/output are done before actually waiting
	// on the cmd to complete.
	// To do so, we use a waitgroup

	handleInput := noop
	if stdinFunc != nil {
		pIn, err := b.cmd.StdinPipe()
		if err != nil {
			return err
		}

		handleInput = func() error {
			defer func() {
				pIn.Close()
				b.wg.Done()
			}()
			return stdinFunc(pIn)
		}
		b.wg.Add(1)
	}

	handleStdout := noop
	if stdoutFunc != nil {
		pOut, err := b.cmd.StdoutPipe()
		if err != nil {
			return err
		}
		handleStdout = func() error {
			defer func() {
				pOut.Close()
				b.wg.Done()
			}()
			return stdoutFunc(pOut)
		}
		b.wg.Add(1)
	}
	handleStderr := noop
	if stderrFunc != nil {
		pErr, err := b.cmd.StderrPipe()
		if err != nil {
			return err
		}
		handleStderr = func() error {
			defer func() {
				pErr.Close()
				b.wg.Done()
			}()
			return stderrFunc(pErr)
		}
		b.wg.Add(1)
	}

	if err := b.cmd.Start(); err != nil {
		return err
	}

	go handleStdout()
	go handleStderr()
	go handleInput()

	return nil
}

// Start does a Cmd.Start on the command
func (b *Builder) Start(args ...interface{}) error {
	if err := b.Prepare(args...); err != nil {
		return err
	}
	return b.StartWithHandlers(nil, nil, nil, args...)
}

// Wait waits for the command to complete
func (b *Builder) Wait() error {
	b.wg.Wait()
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
	if b.dir != "" {
		b.cmd.Dir = b.dir
	}
	if b.stdin != nil {
		b.cmd.Stdin = b.stdin
	}
	if b.stdout != nil {
		b.cmd.Stdout = b.stdout
	}
	if b.stderr != nil {
		b.cmd.Stderr = b.stderr
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
