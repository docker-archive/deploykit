package discovery

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/api/types"
	plugin "github.com/docker/libmachete/plugin/util"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type pluginInstance struct {
	types.Plugin
	client *plugin.Client
}

// Call calls the plugin with some message
func (i *pluginInstance) Call(method, op string, message, result interface{}) ([]byte, error) {
	return i.client.Call(method, op, message, result)
}

// Dir is an object for finding out what plugins we have access to.
type Dir struct {
	dir     string
	plugins map[string]*pluginInstance
	lock    sync.RWMutex
}

// PluginByName returns a plugin by name
func (r *Dir) PluginByName(name string) types.Callable {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if instance, has := r.plugins[name]; has {
		return instance
	}
	return nil
}

// NewDir creates a registry instance with the given file directory path.  The entries in the directory
// are either unix socket files or a flat file indicating the tcp port.
func NewDir(dir string) (*Dir, error) {
	registry := &Dir{
		plugins: map[string]*pluginInstance{},
		dir:     dir,
	}
	return registry, registry.Refresh()
}

// Refresh rescans the driver directory to see what drivers are there.
func (r *Dir) Refresh() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	log.Infoln("Opening:", r.dir)
	entries, err := ioutil.ReadDir(r.dir)
	if err != nil {
		return err
	}

	found := map[string]*pluginInstance{}

entries:
	for _, entry := range entries {
		if !entry.IsDir() {

			var listenerURL string

			if entry.Mode()&os.ModeSocket != 0 {
				listenerURL = "unix://" + filepath.Join(r.dir, entry.Name())
			} else {
				// content is the url, name is the plugin name
				f := filepath.Join(r.dir, entry.Name())
				log.Debugln("reading", f)
				buff, err := ioutil.ReadFile(f)
				if err != nil {
					log.Warningln("cannot read", f)
					continue entries
				}
				listenerURL = string(buff)
			}

			client, err := plugin.NewClient(listenerURL)
			if err != nil {
				log.Warningln("Not valid:", listenerURL, "dir=", r.dir, "file=", entry.Name())
				continue entries
			}

			log.Infoln("Discovered plugin at", listenerURL)
			instance := &pluginInstance{
				Plugin: types.Plugin{
					Name: strings.Split(entry.Name(), ".")[0], // no file extension like .sock
				},
				client: client,
			}
			found[instance.Plugin.Name] = instance
		}
	}

	// now update
	r.plugins = found

	return nil
}
