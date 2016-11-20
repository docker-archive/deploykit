package os

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLaunchOSCommand(t *testing.T) {

	launcher, err := NewLauncher(os.TempDir())
	require.NoError(t, err)

	starting, err := launcher.Launch("no-such-command")
	require.Error(t, err)
	require.Nil(t, starting)

	starting, err = launcher.Launch("sleep", "100")
	require.NoError(t, err)

	<-starting
	t.Log("started")
}

func TestLaunchHasLog(t *testing.T) {

	dir := os.TempDir()
	launcher, err := NewLauncher(dir)
	require.NoError(t, err)

	starting, err := launcher.Launch("sleep", "1 && echo 'hello'")
	require.NoError(t, err)

	err = <-starting
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	buff, err := ioutil.ReadFile(launcher.plugins[plugin("sleep")].log)
	require.NoError(t, err)
	require.Equal(t, "hello\n", string(buff))

}
