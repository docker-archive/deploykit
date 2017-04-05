package metadata

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit.gcp/plugin/gcloud"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"sync"
)

type plugin struct {
	api  gcloud.API
	zone string

	once   sync.Once
	topics map[string]interface{}
}

// NewGCEMetadataPlugin creates a new GCE metadata plugin for a given project
// and zone.
func NewGCEMetadataPlugin(project, zone string) metadata.Plugin {
	api, err := gcloud.New(project, zone)
	if err != nil {
		log.Fatal(err)
	}

	return &plugin{
		api:  api,
		zone: zone,
	}
}

func (p *plugin) buildTopics() map[string]interface{} {
	topics := map[string]interface{}{}

	p.addTopic(topics, "project", p.GetProject)
	p.addTopic(topics, "zone", p.GetZone)

	return topics
}

func (p *plugin) addTopic(topics map[string]interface{}, path string, getter func() string) {
	metadata_plugin.Put(metadata_plugin.Path(path), func() interface{} { return getter() }, topics)
}

// List returns a list of *child nodes* given a path, which is specified as a slice
// where for i > j path[i] is the parent of path[j]
func (p *plugin) List(topic metadata.Path) ([]string, error) {
	p.once.Do(func() { p.topics = p.buildTopics() })

	return types.List(topic, p.topics), nil
}

// Get retrieves the value at path given.
func (p *plugin) Get(topic metadata.Path) (*types.Any, error) {
	p.once.Do(func() { p.topics = p.buildTopics() })

	return types.GetValue(topic, p.topics)
}

func (p *plugin) GetProject() string {
	return p.api.GetProject()

}
func (p *plugin) GetZone() string {
	return p.api.GetZone()
}
