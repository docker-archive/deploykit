package discovery

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/plugin"
)

type dirPluginDiscovery struct {
	dir  string
	lock sync.Mutex
}

// Find returns a plugin by name
func (r *dirPluginDiscovery) Find(name string) (*plugin.Endpoint, error) {

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

// newDirPluginDiscovery creates a registry instance with the given file directory path.
func newDirPluginDiscovery(dir string) (*dirPluginDiscovery, error) {
	d := &dirPluginDiscovery{dir: dir}

	// Perform a dummy read to catch obvious issues early (such as the directory not existing).
	_, err := d.List()
	return d, err
}

func (r *dirPluginDiscovery) dirLookup(entry os.FileInfo) (*plugin.Endpoint, error) {
	if entry.Mode()&os.ModeSocket != 0 {
		socketPath := filepath.Join(r.dir, entry.Name())
		return &plugin.Endpoint{
			Protocol: "unix",
			Address:  socketPath,
			Name:     entry.Name(),
		}, nil
	}

	return nil, fmt.Errorf("File is not a socket: %s", entry)
}

// List returns a list of plugins known, keyed by the name
func (r *dirPluginDiscovery) List() (map[string]*plugin.Endpoint, error) {

	r.lock.Lock()
	defer r.lock.Unlock()

	log.Debugln("Opening:", r.dir)
	entries, err := ioutil.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}

	plugins := map[string]*plugin.Endpoint{}

	for _, entry := range entries {
		if !entry.IsDir() {

			instance, err := r.dirLookup(entry)
			if err != nil || instance == nil {
				log.Warningln("Loading plugin err=", err)
				continue
			}

			log.Debugln("Discovered plugin at", instance.Address)
			plugins[instance.Name] = instance
		}
	}

	return plugins, nil
}
