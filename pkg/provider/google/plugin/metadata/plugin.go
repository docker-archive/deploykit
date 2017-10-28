package metadata

import (
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/provider/google/plugin/gcloud"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

type plugin struct {
	api         gcloud.API
	apiMetadata gcloud.APIMetadata

	once   sync.Once
	topics map[string]interface{}
}

// NewGCEMetadataPlugin creates a new GCE metadata plugin for a given project
// and zone.
func NewGCEMetadataPlugin(project, zone string) metadata.Plugin {
	api, err := gcloud.NewAPI(project, zone)
	if err != nil {
		log.Fatal(err)
	}

	apiMetadata := gcloud.NewAPIMetadata()

	return &plugin{
		api:         api,
		apiMetadata: apiMetadata,
	}
}

func (p *plugin) buildTopics() map[string]interface{} {
	topics := map[string]interface{}{}

	p.addTopic(topics, "project", p.getProject)
	p.addTopic(topics, "zone", p.getZone)

	p.addTopic(topics, "instance/projectID", p.apiMetadata.ProjectID)
	p.addTopic(topics, "instance/numericalProjectID", p.apiMetadata.NumericProjectID)
	p.addTopic(topics, "instance/internalIP", p.apiMetadata.InternalIP)
	p.addTopic(topics, "instance/externalIP", p.apiMetadata.ExternalIP)
	p.addTopic(topics, "instance/hostname", p.apiMetadata.Hostname)
	p.addTopic(topics, "instance/ID", p.apiMetadata.InstanceID)
	p.addTopic(topics, "instance/name", p.apiMetadata.InstanceName)
	p.addTopic(topics, "instance/zone", p.apiMetadata.Zone)
	p.addTopic(topics, "instance/network", p.getNetwork)

	return topics
}

func (p *plugin) addTopic(topics map[string]interface{}, path string, getter func() (string, error)) {
	types.Put(types.PathFromString(path), func() interface{} {
		value, err := getter()
		if err != nil {
			return nil // TODO
		}
		return value
	}, topics)
}

// Keys returns a list of *child nodes* given a path, which is specified as a slice
// where for i > j path[i] is the parent of path[j]
func (p *plugin) Keys(topic types.Path) ([]string, error) {
	p.loadTopics()

	return types.List(topic, p.topics), nil
}

// Get retrieves the value at path given.
func (p *plugin) Get(topic types.Path) (*types.Any, error) {
	p.loadTopics()

	return types.GetValue(topic, p.topics)
}

func (p *plugin) loadTopics() {
	p.once.Do(func() { p.topics = p.buildTopics() })
}

func (p *plugin) getProject() (string, error) {
	return p.api.GetProject(), nil
}

func (p *plugin) getZone() (string, error) {
	return p.api.GetZone(), nil
}

func (p *plugin) getNetwork() (string, error) {
	value, err := p.apiMetadata.Get("instance/network-interfaces/0/network")
	if err != nil {
		return "", err
	}

	return last(value), nil
}

func last(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}
