package discovery

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/plugin"
	"github.com/docker/libmachete/plugin/util"
)

type pluginInstance struct {
	name     string
	endpoint string
	client   *util.Client
}

// String returns a string representation of the callable.
func (i *pluginInstance) String() string {
	return i.endpoint
}

// Call calls the plugin with some message
func (i *pluginInstance) Call(endpoint plugin.Endpoint, message, result interface{}) ([]byte, error) {
	return i.client.Call(endpoint, message, result)
}

// Dir is an object for finding out what plugins we have access to.
type Dir struct {
	dir     string
	plugins map[string]*pluginInstance
	lock    sync.RWMutex
}

// PluginByName returns a plugin by name
func (r *Dir) PluginByName(name string) (plugin.Callable, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if instance, has := r.plugins[name]; has {
		return instance, nil
	}

	// not there. try to load this..
	entry, err := os.Stat(filepath.Join(r.dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Plugin %s is not loaded", name)
		}
		return nil, err
	}

	instance, err := r.dirLookup(entry)
	if err != nil {
		return nil, err
	}

	r.plugins[instance.name] = instance
	return instance, nil
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

func (r *Dir) dirLookup(entry os.FileInfo) (*pluginInstance, error) {
	var listenerURL string
	if entry.Mode()&os.ModeSocket != 0 {
		listenerURL = "unix://" + filepath.Join(r.dir, entry.Name())
	} else {
		// content is the url, name is the plugin name
		f := filepath.Join(r.dir, entry.Name())
		buff, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}
		listenerURL = string(buff)
	}

	client, err := util.NewClient(listenerURL)
	if err != nil {
		log.Warningln("Not valid:", listenerURL, "dir=", r.dir, "file=", entry.Name())
		return nil, err
	}

	return &pluginInstance{
		endpoint: listenerURL,
		name:     strings.Split(entry.Name(), ".")[0], // no file extension like .sock
		client:   client,
	}, nil
}

// Refresh rescans the driver directory to see what drivers are there.
func (r *Dir) Refresh() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	log.Debugln("Opening:", r.dir)
	entries, err := ioutil.ReadDir(r.dir)
	if err != nil {
		return err
	}

	found := map[string]*pluginInstance{}

entries:
	for _, entry := range entries {
		if !entry.IsDir() {

			instance, err := r.dirLookup(entry)
			if err != nil || instance == nil {
				log.Warningln("Loading plugin instance err=", err)
				continue entries
			}

			log.Debugln("Discovered plugin at", instance.endpoint)
			found[instance.name] = instance
		}
	}

	// now update
	r.plugins = found

	return nil
}

// List returns a list of plugins known, keyed by the name
func (r *Dir) List() (map[string]plugin.Callable, error) {
	err := r.Refresh()
	if err != nil {
		return nil, err
	}

	result := map[string]plugin.Callable{}

	for k, v := range r.plugins {
		result[k] = plugin.Callable(v)
	}
	return result, nil
}
