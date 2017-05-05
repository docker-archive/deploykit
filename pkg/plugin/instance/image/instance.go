package image

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/graymeta/stow"
	"github.com/spf13/afero"

	// Blank imports here loads all the supported backends
	_ "github.com/graymeta/stow/google"
	_ "github.com/graymeta/stow/local"
	_ "github.com/graymeta/stow/s3"
)

var log = logutil.New("module", "instance/image")

// URL is a location in url form
type URL string

// Backend is a logical name that can select the different backends configured
// with this plugin.  Each backend corresponds to a location as model by stow.
type Backend string

// Options model the necessary input to create a single dial location
type Options struct {
	// Kind is the kind of backend (e.g. s3, azure)
	Kind string

	// Container is the name of the container, eg. name of the bucket
	Container string

	// Config are the configuration parameters for the image store backend
	Config map[string]*types.Any `json:",omitempty" yaml:",omitempty"`
}

// Spec is the schema for creating an image instance.
// An image instance is a logical unit that has an id and can consist of multiple
// blobs (e.g. one for kernel, one for initrd, each with its own URL.
type Spec struct {

	// Backend as a string indicates the backend e.g. s3, azure, gcp
	// it maps to one of the configurations in the map that the plugin starts with.
	Backend Backend

	// Source are the locations of the content parts to be uploaded. It's a map of a
	// name to an URL for retrieving the content
	Sources map[string]URL
}

// Properties contain data for an instance of an image which can have multiple components
type Properties struct {
	// ID is the instance unique ID
	ID *instance.ID

	// Locations
	Locations map[string]URL `json:",omitempty" yaml:",omitempty"`
}

// NewPlugin creates an instance plugin for images.
func NewPlugin(configs map[Backend]Options, namespace map[string]string) (instance.Plugin, error) {
	p := &imagePlugin{
		backends:   configs,
		containers: map[Backend]stow.Container{},
		fs:         afero.NewOsFs(),
		namespace:  namespace,
		locationFunc: func(kind string, config stow.Config) (stow.Location, error) {
			return stow.Dial(kind, config)
		},
	}
	return p.init()
}

func (p *imagePlugin) init() (instance.Plugin, error) {
	for b, opt := range p.backends {
		configMap := stow.ConfigMap{}
		for k, v := range opt.Config {
			configMap[k] = v.String()
		}

		location, err := p.locationFunc(opt.Kind, configMap)
		if err != nil {
			return nil, err
		}

		container, err := location.Container(opt.Container)
		if err != nil {
			container, err = location.CreateContainer(opt.Container)
			if err != nil {
				return nil, err
			}
		}
		p.containers[b] = container
	}
	return p, nil
}

type imagePlugin struct {
	backends     map[Backend]Options
	containers   map[Backend]stow.Container
	namespace    map[string]string
	fs           afero.Fs
	locationFunc func(kind string, config stow.Config) (stow.Location, error)
}

// Validate performs local validation on a provision request.
func (p imagePlugin) Validate(req *types.Any) error {
	imageSpec := &Spec{}
	if err := req.Decode(imageSpec); err != nil {
		return err
	}

	if len(imageSpec.Sources) == 0 {
		return fmt.Errorf("no sources")
	}

	// get the location
	if _, has := p.containers[imageSpec.Backend]; !has {
		return fmt.Errorf("unknown backend: %s", imageSpec.Backend)
	}

	// check that the files exist.
	for source, sourceURL := range imageSpec.Sources {

		u, err := url.Parse(string(sourceURL))
		if err != nil {
			return err
		}

		if u.Scheme == "" || u.Scheme == "file" {
			exists, err := afero.Exists(p.fs, u.Path)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("not found for %s: %s", source, sourceURL)
			}
		}
	}
	return nil
}

// Provision creates a new instance.
func (p imagePlugin) Provision(spec instance.Spec) (*instance.ID, error) {

	id := instance.ID(fmt.Sprintf("image-%d", time.Now().UnixNano()))

	if spec.Properties == nil {
		return nil, fmt.Errorf("missing properties in spec")
	}

	imageSpec := Spec{}
	if err := spec.Properties.Decode(&imageSpec); err != nil {
		return nil, fmt.Errorf("error decoding guest configuration: %s", spec.Properties.String())
	}

	// get the container
	container, has := p.containers[imageSpec.Backend]
	if !has {
		return nil, fmt.Errorf("unknown backend: %s", imageSpec.Backend)
	}

	// inject additional tags
	spec.Tags["infrakit.id"] = string(id)
	if spec.LogicalID != nil {
		spec.Tags["infrakit.logicalID"] = string(*spec.LogicalID)
	}

	// for each part of the image we upload the contents, and tag with the same metadata
	for part, sourceURL := range imageSpec.Sources {

		u, err := url.Parse(string(sourceURL))
		if err != nil {
			return nil, err
		}

		var content io.ReadCloser

		switch u.Scheme {
		case "", "file":

			f, err := p.fs.Open(u.Path)
			if err != nil {
				return nil, err
			}
			content = f

		case "http", "https":
			client := &http.Client{}
			req, err := http.NewRequest(http.MethodGet, u.String(), nil)
			if err != nil {
				return nil, err
			}
			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			content = resp.Body

		default:
			return nil, fmt.Errorf("unknown protocol: %s", u.Scheme)
		}

		if content == nil {
			return nil, fmt.Errorf("input stream is nil")
		}

		defer content.Close()

		// This is pretty bad... the lib doesn't support streaming so we'd have to read
		// the entire thing into memory so we can compute the size correctly.
		all, err := ioutil.ReadAll(content)
		if err != nil {
			return nil, err
		}

		name := part
		size := len(all)
		buff := bytes.NewBuffer(all)
		fingerprint := types.Fingerprint(types.AnyBytes(all))

		apply := map[string]string{
			"infrakit.fingerprint." + part: fingerprint,
			"infrakit.part." + part:        part,
			"infrakit.source." + part:      string(sourceURL),
			"infrakit.backend." + part:     string(imageSpec.Backend),
		}
		for k, v := range spec.Tags {
			apply[k] = v
		}

		_, mm := mergeTags(apply, p.namespace)
		metadata := map[string]interface{}{}
		for k, v := range mm {
			metadata[k] = v
		}

		item, err := container.Put(name, buff, int64(size), metadata)
		if err != nil {
			return nil, err
		}

		log.Info("Uploaded part", "name", name, "size", size, "metadata", metadata, "item", item)
	}

	return &id, nil
}

// Label labels the instance. This function is a No-op. Images are immutable -- cannot label them after creation
func (p imagePlugin) Label(instance instance.ID, labels map[string]string) error {
	return nil
}

// Destroy terminates an existing instance. This is a No-op. Images are immutable and never go away.
func (p imagePlugin) Destroy(id instance.ID) error {
	return nil
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (p imagePlugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {

	// Group the items by the id tag because a single instance can have multiple components (items).
	items := map[instance.ID][]stow.Item{}

	for _, container := range p.containers {
		err := walkContainer(container, items, p.namespace, tags)
		if err != nil {
			return nil, err
		}
	}
	return toInstances(items, properties), nil
}

// given a container, go through all the items and find those matching the criteria
func walkContainer(container stow.Container, items map[instance.ID][]stow.Item,
	namespace, tags map[string]string) error {

	return stow.Walk(container, "", 100,
		func(item stow.Item, err error) error {

			if err != nil {
				log.Warn("error while walking", "id", container.ID(), "name", container.Name(), "err", err)
				return err
			}

			// check item to match namespace and selector tags
			mm, err := item.Metadata()
			if err != nil {
				log.Warn("error getting metadata", "id", item.ID(), "name", item.Name(), "err", err)
				return nil
			}

			metadata := map[string]string{}
			for k, v := range mm {
				metadata[k] = fmt.Sprintf("%v", v)
			}

			// tags to check
			_, expected := mergeTags(namespace, tags)

			if hasDifferentTags(expected, metadata) {
				log.Debug("no match", "tags", metadata, "id", item.ID(), "name", item.Name())
				return nil
			}

			id, has := metadata["infrakit.id"]
			if !has {
				log.Warn("no id label", "id", item.ID(), "name", item.Name())
				return nil
			}

			instanceID := instance.ID(id)

			if _, has := items[instanceID]; !has {
				items[instanceID] = []stow.Item{}
			}

			items[instanceID] = append(items[instanceID], item)

			return nil
		})
}

// given a map of items grouped by instance id, return a list of instance descriptions
func toInstances(items map[instance.ID][]stow.Item, properties bool) []instance.Description {
	instances := []instance.Description{}

	for id, parts := range items {

		all := []map[string]string{}
		lid := ""
		for _, p := range parts {
			if m, err := p.Metadata(); err == nil {

				mm := map[string]string{}
				for k, v := range m {
					mm[k] = fmt.Sprintf("%v", v)

					if k == "infrakit.logicalID" {
						lid = mm[k]
					}
				}
				all = append(all, mm)
			}
		}

		_, tags := mergeTags(all...)

		var logicalID instance.LogicalID
		if lid != "" {
			logicalID = instance.LogicalID(lid)
		}

		instance := instance.Description{
			ID:        id,
			LogicalID: &logicalID,
			Tags:      tags,
		}

		if properties {

			// from each item, get its actual URL and reconstruct the spec...
			locations := map[string]URL{}

			for _, p := range parts {
				locations[p.Name()] = URL(p.URL().String())
			}

			prop := Properties{
				ID:        &id,
				Locations: locations,
			}

			if any, err := types.AnyValue(&prop); err == nil {
				instance.Properties = any
			}
		}

		instances = append(instances, instance)
	}
	return instances
}

// mergeTags merges multiple maps of tags, implementing 'last write wins' for colliding keys.
// Returns a sorted slice of all keys, and the map of merged tags.  Sorted keys are particularly useful to assist in
// preparing predictable output such as for tests.
func mergeTags(tagMaps ...map[string]string) ([]string, map[string]string) {
	keys := []string{}
	tags := map[string]string{}
	for _, tagMap := range tagMaps {
		for k, v := range tagMap {
			if _, exists := tags[k]; exists {
				log.Warn("Overwriting tag value", "key", k)
			} else {
				keys = append(keys, k)
			}
			tags[k] = v
		}
	}
	sort.Strings(keys)
	return keys, tags
}

func hasDifferentTags(expected, actual map[string]string) bool {
	if len(actual) == 0 {
		return true
	}
	for k, v := range expected {
		if a, ok := actual[k]; ok && a != v {
			return true
		}
	}

	return false
}
