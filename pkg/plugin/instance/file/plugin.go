package file

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/afero"
	"math/rand"
)

var (
	log = logutil.New("module", "launch/os")
)

// This example uses local files as a representation of an instance.  When we
// create an instance, we write a file in a directory.  The content of the file is simply
// the message in the provision spec, so we can verify correctness of the content easily.
// When we destroy an instance, we remove the file.
// DescribeInstances simply would list the files with the matching
// tags.

// Spec is just whatever that can be unmarshalled into a generic JSON map
type Spec map[string]interface{}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// fileInstance represents a single file instance on disk.
type fileInstance struct {
	instance.Description
	Spec instance.Spec
}

type plugin struct {
	Dir string
	fs  afero.Fs
}

// NewPlugin returns an instance plugin backed by disk files.
func NewPlugin(dir string) instance.Plugin {
	log.Debug("file instance plugin", "dir", dir)
	return &plugin{
		Dir: dir,
		fs:  afero.NewOsFs(),
	}
}

// Info returns a vendor specific name and version
func (p *plugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-file",
			Version: "0.3.0",
		},
		URL: "https://github.com/docker/infrakit",
	}
}

// ExampleProperties returns the properties / config of this plugin
func (p *plugin) ExampleProperties() *types.Any {
	any, err := types.AnyValue(Spec{
		"exampleString": "a_string",
		"exampleBool":   true,
		"exampleInt":    1,
	})
	if err != nil {
		return nil
	}
	return any
}

// Validate performs local validation on a provision request.
func (p *plugin) Validate(req *types.Any) error {
	log.Debug("validate", "req", req)

	spec := Spec{}
	if err := req.Decode(&spec); err != nil {
		return err
	}

	log.Debug("Validated", "spec", spec)
	return nil
}

// Provision creates a new instance based on the spec.
func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {
	// simply writes a file
	// use timestamp as instance id
	id := instance.ID(fmt.Sprintf("instance-%d", rand.Int63()))
	buff, err := json.MarshalIndent(fileInstance{
		Description: instance.Description{
			Tags:      spec.Tags,
			ID:        id,
			LogicalID: spec.LogicalID,
		},
		Spec: spec,
	}, "", "")
	log.Debug("provision", "id", id, "data", string(buff), "err", err)
	if err != nil {
		return nil, err
	}
	return &id, afero.WriteFile(p.fs, filepath.Join(p.Dir, string(id)), buff, 0644)
}

// Label labels the instance
func (p *plugin) Label(instance instance.ID, labels map[string]string) error {
	fp := filepath.Join(p.Dir, string(instance))
	buff, err := afero.ReadFile(p.fs, fp)
	if err != nil {
		return err
	}
	instanceData := fileInstance{}
	err = json.Unmarshal(buff, &instanceData)
	if err != nil {
		return err
	}

	if instanceData.Description.Tags == nil {
		instanceData.Description.Tags = map[string]string{}
	}
	for k, v := range labels {
		instanceData.Description.Tags[k] = v
	}

	buff, err = json.MarshalIndent(instanceData, "", "")
	if err != nil {
		return err
	}
	return afero.WriteFile(p.fs, fp, buff, 0644)
}

// Destroy terminates an existing instance.
func (p *plugin) Destroy(instance instance.ID, context instance.Context) error {
	fp := filepath.Join(p.Dir, string(instance))
	log.Debug("destroy", "path", fp)
	return p.fs.Remove(fp)
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
// TODO - need to define the fitlering of tags => AND or OR of matches?
func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	log.Debug("describe-instances", "tags", tags)
	entries, err := afero.ReadDir(p.fs, p.Dir)
	if err != nil {
		return nil, err
	}

	result := []instance.Description{}
scan:
	for _, entry := range entries {
		fp := filepath.Join(p.Dir, entry.Name())
		file, err := p.fs.Open(fp)
		if err != nil {
			log.Warn("Error opening file", "path", fp)
			continue scan
		}

		inst := fileInstance{}
		err = json.NewDecoder(file).Decode(&inst)
		if err != nil {
			log.Warn("Cannot decode", "file", entry.Name())
			continue scan
		}

		if properties {
			if blob, err := types.AnyValue(inst.Spec); err == nil {
				inst.Properties = blob
			}
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
