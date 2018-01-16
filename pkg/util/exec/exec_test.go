package exec

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/docker/infrakit/pkg/testing"
	"github.com/stretchr/testify/require"
)

var (
	busybox = Command(`
docker run --rm \
       busybox {{ call }} {{ argv | join " "}}
`)
	busyboxHostname = Command(`
docker run --rm \
       busybox hostname
`)
	busyboxLs = Command(`docker run --rm \
       busybox ls {{ arg 1 }}
`)

	busyboxSh = Command(`docker run --rm -ti --name {{ .container }} busybox /bin/sh`)

	dockerStop = Command(`docker stop {{ arg 1 }}`)

	dateStream = Command(`docker run --rm --name {{ arg "container" }} chungers/timer streamer`)
)

func TestBuilder(t *testing.T) {

	b := busyboxHostname
	cmd, err := b.generate()
	require.NoError(t, err)
	require.Equal(t, []string{"docker", "run", "--rm", "busybox", "hostname"}, cmd)

	b = busybox.WithFunc("call", func() string { return "ls -al" })
	cmd, err = b.generate("sys", "var")
	require.NoError(t, err)
	require.Equal(t, []string{"docker", "run", "--rm", "busybox", "ls", "-al", "sys", "var"}, cmd)

	b = busyboxLs
	cmd, err = b.generate("sys", "var")
	require.NoError(t, err)
	require.Equal(t, []string{"docker", "run", "--rm", "busybox", "ls", "sys"}, cmd)

	b = busyboxSh.WithContext(map[string]interface{}{"container": "bob"})
	cmd, err = b.generate()
	require.NoError(t, err)
	require.Equal(t, []string{"docker", "run", "--rm", "-ti", "--name", "bob", "busybox", "/bin/sh"}, cmd)

}

func TestStartWithHandlersError(t *testing.T) {
	// Testing command that fails to start, and makes sure Wait() doesn't deadlock
	command := Command("pwd").InheritEnvs(true).
		WithDir("/impossibledir*&$%#")
	err := command.StartWithHandlers(
		nil,
		func(stdout io.Reader) error {
			T(100).Info("reading from stdout")
			return nil
		},
		func(stderr io.Reader) error {
			T(100).Info("reading from stderr")
			return nil
		})
	// Bad directory error
	require.Error(t, err)

	// Command not started error
	err = command.Wait()
	require.Error(t, err)
}

func TestRunDocker(t *testing.T) {

	if SkipTests("docker") {
		t.SkipNow()
	}

	output, err := busyboxHostname.Output()
	require.NoError(t, err)
	require.True(t, len(output) > 0)

	output, err = busybox.WithFunc("call", func() string { return "whoami" }).Output()
	require.NoError(t, err)
	require.Equal(t, "root\n", string(output))

	output, err = busyboxLs.Output("var")
	require.NoError(t, err)
	require.Equal(t, []string{"spool", "www"}, strings.Split(strings.Trim(string(output), " \n"), "\n"))

	name := "stream-test"
	go func() {
		<-time.After(2 * time.Second)
		err := dockerStop.Start(name)
		if err != nil {
			panic(err)
		}
		dockerStop.Wait()
	}()

	err = dateStream.InheritEnvs(true).WithArg("container", name).
		WithStdout(os.Stderr).
		WithStderr(os.Stderr).
		Start()
	require.NoError(t, err)

	// testing with stdin
	err = Command("/bin/sh").InheritEnvs(true).
		WithStdout(os.Stderr).
		WithStderr(os.Stderr).
		StartWithHandlers(
			func(stdin io.Writer) error {
				T(100).Info("about to write to stdin")
				stdin.Write([]byte(`for i in $(seq 10); do echo $i; sleep 1; done`))
				T(100).Info("wrote to stdin")
				return nil
			}, nil, nil)
	require.NoError(t, err)
}

func TestPipeline1(t *testing.T) {

	source := Command("echo hello")
	stage1 := Command("/bin/cat -b -n")

	source.Prepare()
	stage1.Prepare()

	require.NotNil(t, source.cmd)
	require.NotNil(t, stage1.cmd)

	source.Stdin(func(w io.Writer) error { return nil })
	source.StdoutTo(stage1)
	stage1.Stdout(os.Stdout)

	source.cmd.Start()
	stage1.cmd.Start()

	source.cmd.Wait()
}

func TestPipeline2(t *testing.T) {
	// testing with stdin

	source := Command("/bin/sh")
	stage1 := Command("/bin/cat -b -n")

	source.Prepare()
	stage1.Prepare()

	err := source.Stdin(func(w io.Writer) error {
		T(100).Info("**** about to write")
		w.Write([]byte(`for i in $(seq 10); do echo $i; sleep 1; done;`))
		T(100).Info("written input")
		w.Write([]byte(`exit`))
		T(100).Info("sent exit")
		return nil
	})
	require.NoError(t, err)

	source.StdoutTo(stage1)
	stage1.Stdout(os.Stderr)

	source.cmd.Start()
	stage1.cmd.Start()

	T(100).Info("wait on source")
	source.cmd.Wait()
}
