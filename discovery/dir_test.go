package discovery

import (
	"fmt"
	"github.com/docker/infrakit/plugin/util"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDirDiscovery(t *testing.T) {

	dir := fmt.Sprintf("/tmp/plugins/%d", time.Now().Unix())
	err := os.MkdirAll(dir, 0777)
	require.NoError(t, err)

	name1 := "test-tcp-server"
	listen1 := "tcp://:4321" + filepath.Join(dir, name1)
	stop1, errors1, err1 := util.StartServer(listen1, mux.NewRouter())
	require.NoError(t, err1)
	require.NotNil(t, stop1)
	require.NotNil(t, errors1)

	name2 := "test-unix-server"
	listen2 := "unix://" + filepath.Join(dir, name2+".sock")
	stop2, errors2, err2 := util.StartServer(listen2, mux.NewRouter())
	require.NoError(t, err2)
	require.NotNil(t, stop2)
	require.NotNil(t, errors2)

	discover, err := NewDir(dir)
	require.NoError(t, err)

	p, err := discover.PluginByName(name1)
	require.NoError(t, err)
	require.NotNil(t, p)

	p, err = discover.PluginByName(name2)
	require.NoError(t, err)
	require.NotNil(t, p)

	// Now we stop the servers
	close(stop1)

	// wait for socket file to disappear
	time.Sleep(100 * time.Millisecond)

	err = discover.Refresh()
	require.Nil(t, err)

	p, err = discover.PluginByName(name1)
	require.Error(t, err)

	p, err = discover.PluginByName(name2)
	require.NoError(t, err)
	require.NotNil(t, p)

	close(stop2)

	// wait for socket file to disappear
	time.Sleep(100 * time.Millisecond)

	err = discover.Refresh()
	require.Nil(t, err)

	p, err = discover.PluginByName(name1)
	require.Error(t, err)

	p, err = discover.PluginByName(name2)
	require.Error(t, err)

	list, err := discover.List()
	require.NoError(t, err)
	require.Equal(t, 0, len(list))
}
