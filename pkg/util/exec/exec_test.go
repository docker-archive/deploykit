package exec

import (
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
	busyboxDateStream Command = `docker run --rm --name {{ .container }} \
busybox /bin/sh -c 'while true; do date; sleep {{ .sleep }}; done'`

	dockerStop Command = `docker stop {{ arg 1 }}`
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

	b = busyboxDateStream.builder().WithContext(map[string]interface{}{"container": "bob", "sleep": "1"})
	cmd, err = b.generate()
	require.NoError(t, err)
	require.Equal(t, []string{"docker", "run", "--rm", "--name", "bob", "busybox", "/bin/sh", "-c",
		"'while", "true;", "do", "date;", "sleep", "1;", "done'"}, cmd)

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
	err = busyboxDateStream.WithContext(map[string]interface{}{"container": name, "sleep": 1}).Start()
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	err = dockerStop.Run(name)
	require.NoError(t, err)
}
