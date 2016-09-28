package discovery

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
	dir  string
	lock sync.Mutex
}

// PluginByName returns a plugin by name
func (r *Dir) PluginByName(name string) (plugin.Callable, error) {

	plugins, err := r.List()
	if err != nil {
		return nil, err
	}

	p, exists := plugins[name]
	if !exists {
		return nil, fmt.Errorf("Plugin not found: %s", name)
	}

	return p, nil
}

// NewDir creates a registry instance with the given file directory path.  The entries in the directory
// are either unix socket files or a flat file indicating the tcp port.
func NewDir(dir string) (*Dir, error) {
	d := &Dir{dir: dir}

	// Perform a dummy read to catch obvious issues early (such as the directory not existing).
	_, err := d.List()
	return d, err
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

// List returns a list of plugins known, keyed by the name
func (r *Dir) List() (map[string]plugin.Callable, error) {

	r.lock.Lock()
	defer r.lock.Unlock()

	log.Debugln("Opening:", r.dir)
	entries, err := ioutil.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}

	plugins := map[string]plugin.Callable{}

	for _, entry := range entries {
		if !entry.IsDir() {

			instance, err := r.dirLookup(entry)
			if err != nil || instance == nil {
				log.Warningln("Loading plugin err=", err)
				continue
			}

			log.Debugln("Discovered plugin at", instance.endpoint)
			plugins[instance.name] = plugin.Callable(instance)
		}
	}

	return plugins, nil
}
