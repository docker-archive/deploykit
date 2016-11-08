package manager

import (
	"encoding/json"
	"path/filepath"

	"github.com/docker/infrakit/plugin/group/types"
	"github.com/docker/infrakit/spi/group"
	"github.com/spf13/afero"
)

type PluginSpec struct {
	Plugin     string
	Properties *json.RawMessage
}

type GlobalSpec struct {
	Groups map[group.ID]PluginSpec
}

func (config GlobalSpec) findPlugins() []string {
	// determine list of all plugins by config
	names := map[string]bool{}

	for _, plugin := range config.Groups {

		names[plugin.Plugin] = true

		// Try to parse the properties and if the plugin is a default group plugin then we can
		// determine the flavor and instance plugin names.
		if plugin.Properties != nil {
			spec := &types.Spec{}
			if err := json.Unmarshal([]byte(*plugin.Properties), spec); err == nil {

				if spec.Instance.Plugin != "" {
					names[spec.Instance.Plugin] = true
				}
				if spec.Flavor.Plugin != "" {
					names[spec.Flavor.Plugin] = true
				}
			}
		}
	}

	keys := []string{}
	for k := range names {
		keys = append(keys, k)
	}

	return keys
}

// ReadFileTree populates the receiver with data from the file system tree.
func (g *GlobalSpec) ReadFileTree(fs afero.Fs) error {

	af := &afero.Afero{Fs: fs}
	groups, err := af.ReadDir("Groups")
	if err != nil {
		return err
	}

	if g.Groups == nil {
		g.Groups = map[group.ID]PluginSpec{}
	}

	for _, f := range groups {

		if !f.IsDir() {
			continue
		}

		dir := filepath.Join("Groups", f.Name())
		plugin, err := af.ReadFile(filepath.Join(dir, "Plugin"))
		if err != nil {
			plugin = []byte("") // not set -- this will be modified later on when user watches or updates
		}
		properties, err := af.ReadFile(filepath.Join(dir, "Properties.json"))
		if err != nil {
			return err
		}

		raw := json.RawMessage(properties)
		g.Groups[group.ID(f.Name())] = PluginSpec{
			Plugin:     string(plugin),
			Properties: &raw,
		}
	}

	return nil
}

// WriteFileTree dumps this document in a file system representation
func (g GlobalSpec) WriteFileTree(fs afero.Fs) error {

	if g.Groups == nil {
		return nil
	}

	af := &afero.Afero{Fs: fs}

	// Make a Groups subdirectory
	groupDir := "Groups"
	err := fs.MkdirAll(groupDir, 0700)
	if err != nil {
		return err
	}

	// For each key in the group, create a directory with a config file
	// call config.json that stores the content
	for id, plugin := range g.Groups {

		namedGroup := filepath.Join(groupDir, string(id))
		namedPlugin := filepath.Join(namedGroup, "Plugin")
		namedProperties := filepath.Join(namedGroup, "Properties.json")

		err = fs.MkdirAll(namedGroup, 0700)
		if err != nil {
			return err
		}

		// TODO(chunger) -- this is unfortunate...  the payload to working with group (the spec)
		// is in the Properties attribute but we also have to have a Plugin field in the aggregate format
		// because we can't also just default to a group plugin currently running (e.g. there may be
		// other group plugins running). So this makes for an awkward file system layout.

		err = af.WriteFile(namedPlugin, []byte(plugin.Plugin), 0600)
		if err != nil {
			return err
		}

		if plugin.Properties != nil {
			err = af.WriteFile(namedProperties, []byte(*plugin.Properties), 0600)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
