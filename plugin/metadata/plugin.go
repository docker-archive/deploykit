package metadata

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit.gcp/plugin/gcloud"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

type plugin struct {
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
		topics: buildTopics(api),
	}
}

func buildTopics(api gcloud.API) map[string]interface{} {
	topics := map[string]interface{}{}

	metadata_plugin.Put(metadata_plugin.Path("path/1"),
		func() interface{} {
			return "value1"
		},
		topics)

	metadata_plugin.Put(metadata_plugin.Path("path/2"),
		func() interface{} {
			return "value2"
		},
		topics)

	return topics
}

// List returns a list of *child nodes* given a path, which is specified as a slice
// where for i > j path[i] is the parent of path[j]
func (p *plugin) List(topic metadata.Path) ([]string, error) {
	return types.List(topic, p.topics), nil
}

// Get retrieves the value at path given.
func (p *plugin) Get(topic metadata.Path) (*types.Any, error) {
	return types.GetValue(topic, p.topics)
}
