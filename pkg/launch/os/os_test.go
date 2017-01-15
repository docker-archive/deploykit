package os

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestLaunchOSCommand(t *testing.T) {

	launcher, err := NewLauncher()
	require.NoError(t, err)

	starting, err := launcher.Exec("sleepPlugin", types.AnyValueMust(&LaunchConfig{
		Cmd: "sleep 100",
	}))
	require.NoError(t, err)

	<-starting
	t.Log("started")
}

func TestLaunchWithLog(t *testing.T) {

	logfile := filepath.Join(os.TempDir(), fmt.Sprintf("os-test-%v", time.Now().Unix()))

	launcher, err := NewLauncher()
	require.NoError(t, err)

	starting, err := launcher.Exec("echoPlugin", types.AnyValueMust(&LaunchConfig{
		Cmd:      fmt.Sprintf("echo hello > %s 2>&1", logfile),
		SamePgID: true,
	}))
	require.NoError(t, err)

	<-starting
	t.Log("started")

	time.Sleep(500 * time.Millisecond)

	v, err := ioutil.ReadFile(logfile)
	require.NoError(t, err)
	require.Equal(t, "hello\n", string(v))
}
