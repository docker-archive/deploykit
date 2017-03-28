package exec

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	testutil "github.com/docker/infrakit/pkg/testing"
	"github.com/stretchr/testify/require"
)

const (
	busybox Command = `
docker run --rm \
       busybox {{ call }} {{ argv | join " "}}
`
	busyboxHostname Command = `
docker run --rm \
       busybox hostname
`
	busyboxLs Command = `docker run --rm \
       busybox ls {{ arg 1 }}
`
	busyboxSh Command = `docker run --rm -ti --name {{ .container }} busybox /bin/sh`

	dockerStop Command = `docker stop {{ arg 1 }}`

	dateStream Command = `docker run --rm --name {{ arg 1 }} chungers/timer streamer`
)

func TestBuilder(t *testing.T) {

	b := busyboxHostname.builder()
	cmd, err := b.generate()
	require.NoError(t, err)
	require.Equal(t, []string{"docker", "run", "--rm", "busybox", "hostname"}, cmd)

	b = busybox.builder().WithFunc("call", func() string { return "ls -al" })
	cmd, err = b.generate("sys", "var")
	require.NoError(t, err)
	require.Equal(t, []string{"docker", "run", "--rm", "busybox", "ls", "-al", "sys", "var"}, cmd)

	b = busyboxLs.builder()
	cmd, err = b.generate("sys", "var")
	require.NoError(t, err)
	require.Equal(t, []string{"docker", "run", "--rm", "busybox", "ls", "sys"}, cmd)

	b = busyboxSh.builder().WithContext(map[string]interface{}{"container": "bob"})
	cmd, err = b.generate()
	require.NoError(t, err)
	require.Equal(t, []string{"docker", "run", "--rm", "-ti", "--name", "bob", "busybox", "/bin/sh"}, cmd)

}

func TestRun(t *testing.T) {

	if testutil.SkipTests("docker") {
		t.SkipNow()
	}

	output, err := busyboxHostname.Output()
	require.NoError(t, err)
	require.True(t, len(output) > 0)

	output, err = busybox.builder().WithFunc("call", func() string { return "whoami" }).Output()
	require.NoError(t, err)
	require.Equal(t, "root\n", string(output))

	output, err = busyboxLs.Output("var")
	require.NoError(t, err)
	require.Equal(t, []string{"spool", "www"}, strings.Split(strings.Trim(string(output), " \n"), "\n"))

	name := "stream-test"
	go func() {
		<-time.After(2 * time.Second)
		err := dockerStop.Run(name)
		if err != nil {
			panic(err)
		}
	}()

	err = dateStream.InheritEnvs(true).StartWithStreams(MergeOutput(os.Stderr),
		name, // arg 1 for container name
	)
	require.NoError(t, err)

	// testing with stdin
	err = Command("/bin/sh").InheritEnvs(true).StartWithStreams(
		Do(SendInput(
			func(stdin io.WriteCloser) error {
				stdin.Write([]byte(`for i in $(seq 10); do echo $i; sleep 1; done`))
				return nil
			})).Then(MergeOutput(os.Stderr)).Done(),
		name, // arg 1 for container name
	)
	require.NoError(t, err)

}
