package os

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/launch"
	"github.com/stretchr/testify/require"
)

func TestLaunchOSCommand(t *testing.T) {

	launcher, err := NewLauncher(os.TempDir())
	require.NoError(t, err)

	raw := &launch.Config{}
	err = raw.Marshal(&LaunchConfig{
		Cmd: "no-such-command",
	})
	require.NoError(t, err)

	starting, err := launcher.Exec("badPlugin", raw)
	require.Error(t, err)
	require.Nil(t, starting)

	err = raw.Marshal(&LaunchConfig{
		Cmd:  "sleep",
		Args: []string{"100"},
	})
	require.NoError(t, err)
	starting, err = launcher.Exec("sleepPlugin", raw)
	require.NoError(t, err)

	<-starting
	t.Log("started")
}

func TestLaunchHasLog(t *testing.T) {

	dir := os.TempDir()
	launcher, err := NewLauncher(dir)
	require.NoError(t, err)

	raw := &launch.Config{}
	err = raw.Marshal(&LaunchConfig{
		Cmd:  "sleep",
		Args: []string{"1 && echo 'hello'"},
	})
	require.NoError(t, err)

	starting, err := launcher.Exec("sleepPlugin", raw)
	require.NoError(t, err)

	err = <-starting
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	buff, err := ioutil.ReadFile(launcher.plugins["sleepPlugin"].log)
	require.NoError(t, err)
	require.Equal(t, "hello\n", string(buff))

}
