package discovery

import (
	"github.com/docker/libmachete/plugin/util"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func blockWhileFileExists(name string) {
	for {
		_, err := os.Stat(name)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDirDiscovery(t *testing.T) {

	dir, err := ioutil.TempDir("", "infrakit_dir_test")
	require.NoError(t, err)

	name1 := "test-tcp-server"
	path1 := filepath.Join(dir, name1)
	listen1 := "tcp://:4321" + path1
	stop1, errors1, err1 := util.StartServer(listen1, mux.NewRouter())
	require.NoError(t, err1)
	require.NotNil(t, stop1)
	require.NotNil(t, errors1)

	name2 := "test-unix-server"
	path2 := filepath.Join(dir, name2+".sock")
	listen2 := "unix://" + path2
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
	blockWhileFileExists(path1)

	p, err = discover.PluginByName(name1)
	require.Error(t, err)

	p, err = discover.PluginByName(name2)
	require.NoError(t, err)
	require.NotNil(t, p)

	close(stop2)

	blockWhileFileExists(path2)

	p, err = discover.PluginByName(name1)
	require.Error(t, err)

	p, err = discover.PluginByName(name2)
	require.Error(t, err)

	list, err := discover.List()
	require.NoError(t, err)
	require.Equal(t, 0, len(list))
}
