package os

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestLaunchOSCommand(t *testing.T) {
	sleepCmd := "sleep 100"
	if runtime.GOOS == "windows" {
		sleepCmd = "timeout 100"
	}
	launcher, err := NewLauncher("os")
	require.NoError(t, err)

	_, starting, err := launcher.Exec("sleepPlugin", plugin.Name("sleepPlugin"), types.AnyValueMust(&LaunchConfig{
		Cmd: sleepCmd,
	}))
	require.NoError(t, err)

	<-starting
	t.Log("started")
}

func TestLaunchWithLog(t *testing.T) {

	logfile := filepath.Join(os.TempDir(), fmt.Sprintf("os-test-%v", time.Now().Unix()))

	launcher, err := NewLauncher("os")
	require.NoError(t, err)

	_, starting, err := launcher.Exec("echoPlugin", plugin.Name("echoPlugin"), types.AnyValueMust(&LaunchConfig{
		Cmd:      fmt.Sprintf("echo hello > %s 2>&1", logfile),
		SamePgID: true,
	}))
	require.NoError(t, err)

	<-starting
	t.Log("started")

	time.Sleep(500 * time.Millisecond)

	v, err := ioutil.ReadFile(logfile)
	require.NoError(t, err)
	require.Equal(t, "hello", strings.TrimSpace(string(v)))
}
