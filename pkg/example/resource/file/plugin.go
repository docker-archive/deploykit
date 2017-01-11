package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"math/rand"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/spf13/afero"
)

// This example uses local files as a representation of a resource.  When we
// create a resource, we write a file in a directory.  The content of the file is simply
// the message in the provision spec, so we can verify correctness of the content easily.
// When we destroy a resource, we remove the file.
// DescribeResources simply would list the files with the matching
// tags.

// Spec is just whatever that can be unmarshalled into a generic JSON map
type Spec map[string]interface{}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// fileResource represents a single file resource on disk.
type fileResource struct {
	resource.Description
	Spec resource.Spec
}

type plugin struct {
	Dir string
	fs  afero.Fs
}

// NewFileResourcePlugin returns a resource plugin backed by disk files.
func NewFileResourcePlugin(dir string) resource.Plugin {
	log.Debugln("file resource plugin. dir=", dir)
	return &plugin{
		Dir: dir,
		fs:  afero.NewOsFs(),
	}
}

// Info returns a vendor specific name and version
func (p *plugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-resource-file",
			Version: "0.1.0",
		},
		URL: "https://github.com/docker/infrakit",
	}
}

// ExampleProperties returns the properties / config of this plugin
func (p *plugin) ExampleProperties() *json.RawMessage {
	buff, err := json.MarshalIndent(Spec{
		"exampleString": "a_string",
		"exampleBool":   true,
		"exampleInt":    1,
	}, "  ", "  ")
	if err != nil {
		panic(err)
	}
	raw := json.RawMessage(buff)
	return &raw
}

// Validate performs local validation on a provision request.
func (p *plugin) Validate(resourceType string, req json.RawMessage) error {
	log.Debugln("validate", string(req))

	spec := Spec{}
	if err := json.Unmarshal(req, &spec); err != nil {
		return err
	}

	log.Debugln("Validated:", spec)
	return nil
}

// Provision creates a new resource based on the spec.
func (p *plugin) Provision(spec resource.Spec) (*resource.ID, error) {
	// simply writes a file
	// use timestamp as resource id
	id := resource.ID(fmt.Sprintf("resource-%d", rand.Int63()))
	buff, err := json.MarshalIndent(fileResource{
		Description: resource.Description{
			Tags: spec.Tags,
			ID:   id,
		},
		Spec: spec,
	}, "  ", "  ")
	log.Debugln("provision", id, "data=", string(buff), "err=", err)
	if err != nil {
		return nil, err
	}
	return &id, afero.WriteFile(p.fs, filepath.Join(p.Dir, string(id)), buff, 0644)
}

// Destroy terminates an existing resource.
func (p *plugin) Destroy(resourceType string, resource resource.ID) error {
	fp := filepath.Join(p.Dir, string(resource))
	log.Debugln("destroy", fp)
	return p.fs.Remove(fp)
}

// DescribeResources returns descriptions of all resources matching all of the provided tags.
// TODO - need to define the fitlering of tags => AND or OR of matches?
func (p *plugin) DescribeResources(resourceType string, tags map[string]string) ([]resource.Description, error) {
	log.Debugln("describe-resources", tags)
	entries, err := afero.ReadDir(p.fs, p.Dir)
	if err != nil {
		return nil, err
	}

	result := []resource.Description{}
scan:
	for _, entry := range entries {
		fp := filepath.Join(p.Dir, entry.Name())
		file, err := p.fs.Open(fp)
		if err != nil {
			log.Warningln("error opening", fp)
			continue scan
		}

		inst := fileResource{}
		err = json.NewDecoder(file).Decode(&inst)
		if err != nil {
			log.Warning("cannot decode", entry.Name())
			continue scan
		}

		if len(tags) == 0 {
			result = append(result, inst.Description)
		} else {
			for k, v := range tags {
				if inst.Tags[k] != v {
					continue scan // we implement AND
				}
			}
			result = append(result, inst.Description)
		}

	}
	return result, nil
}
