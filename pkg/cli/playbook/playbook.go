package playbook

import (
	"io/ioutil"
	"os"

	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/types"
)

// Playbook contains information about the source and a local cache.
type Playbook struct {

	// Source is the original location
	Source string

	// Cache is the cached location, as a url of file:// format
	Cache string
}

// Playbooks is a collection of playbooks indexed by operation
type Playbooks map[Op]*Playbook

// Empty returns true if there are no entries
func (pb Playbooks) Empty() bool {
	return len(pb) == 0
}

// Source returns the source of the operation
func (pb Playbooks) Source(op Op) string {
	if p, has := pb[op]; has {
		return p.Source
	}
	return ""
}

// Add adds an entry
func (pb *Playbooks) Add(op Op, source, cacheDir string) {
	(*pb)[op] = &Playbook{
		Source: source,
		Cache:  cacheDir,
	}
}

// Remove removes the entry
func (pb *Playbooks) Remove(op Op) {
	delete(*pb, op)
}

// Has returns true if op is in the playbooks
func (pb *Playbooks) Has(op Op) bool {
	_, has := pb.Modules()[op]
	return has
}

// Visit visits the entries
func (pb *Playbooks) Visit(f func(Op, Playbook)) {
	for op, b := range *pb {
		f(op, *b)
	}
}

// Modules returns the Modules
func (pb *Playbooks) Modules() map[Op]SourceURL {
	module := map[Op]SourceURL{}
	for k, p := range *pb {
		if p.Cache != "" {
			module[k] = SourceURL(p.Cache)
		} else {
			module[k] = SourceURL(p.Source)
		}
	}
	return module
}

// Save saves the playbooks
func (pb *Playbooks) Save() error {
	return pb.writeTo(local.Playbooks())
}

func (pb *Playbooks) writeTo(path string) error {
	any, err := types.AnyValue(*pb)
	if err != nil {
		return err
	}
	buff, err := any.MarshalYAML()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, buff, 0755)
}

func (pb *Playbooks) loadFrom(path string) error {
	buff, err := ioutil.ReadFile(local.Playbooks())
	if err != nil {
		if !os.IsExist(err) {
			return nil
		}
		return err
	}
	yaml, err := types.AnyYAML(buff)
	if err != nil {
		return err
	}
	return yaml.Decode(pb)
}

// Load loads the playbook
func Load() (*Playbooks, error) {
	pb := &Playbooks{}
	return pb, pb.loadFrom(local.Playbooks())
}
