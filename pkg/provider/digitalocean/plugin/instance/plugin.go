package instance

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/digitalocean/godo"
	instance_types "github.com/docker/infrakit/pkg/provider/digitalocean/plugin/instance/types"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// Spec is just whatever that can be unmarshalled into a generic JSON map
type Spec map[string]interface{}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

type dropletsService interface {
	List(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)
	Get(context.Context, int) (*godo.Droplet, *godo.Response, error)
	Create(context.Context, *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error)
	Delete(context.Context, int) (*godo.Response, error)
}

type tagsService interface {
	TagResources(context.Context, string, *godo.TagResourcesRequest) (*godo.Response, error)
}

type keysService interface {
	List(context.Context, *godo.ListOptions) ([]godo.Key, *godo.Response, error)
}

type plugin struct {
	droplets      dropletsService
	tags          tagsService
	keys          keysService
	namespaceTags map[string]string
}

// NewDOInstancePlugin creates a new DigitalOcean instance plugin for a given region.
func NewDOInstancePlugin(client *godo.Client, namespace map[string]string) instance.Plugin {
	return &plugin{
		droplets:      client.Droplets,
		tags:          client.Tags,
		keys:          client.Keys,
		namespaceTags: namespace,
	}
}

// Info returns a vendor specific name and version
func (p *plugin) VendorInfo() *spi.VendorInfo {
	// FIXME(vdemeester) extract that in a version package
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-digitalocean",
			Version: "0.1.0",
		},
		URL: "https://github.com/docker/infrakit/pkg/provider/digitalocean",
	}
}

// Validate performs local validation on a provision request.
func (p *plugin) Validate(req *types.Any) error {
	log.Debugln("validate", req.String())

	spec := Spec{}
	if err := req.Decode(&spec); err != nil {
		return err
	}

	log.Debugln("Validated:", spec)
	return nil
}

// Label labels the instance
func (p *plugin) Label(instance instance.ID, labels map[string]string) error {
	log.Debugf("label instance %s with %v", instance, labels)

	for key, value := range labels {
		tag := strings.Replace(fmt.Sprintf("%s:%s", key, value), ".", "::", -1)
		_, err := p.tags.TagResources(context.TODO(), tag, &godo.TagResourcesRequest{
			Resources: []godo.Resource{
				{
					ID:   string(instance),
					Type: godo.DropletResourceType,
				},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Provision creates a new instance based on the spec.
func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {
	properties, err := instance_types.ParseProperties(spec.Properties)
	if err != nil {
		return nil, err
	}

	// the basic request is already part of the input
	dropletCreateRequest := properties.DropletCreateRequest

	// Some computed overrides:

	// the name must be given suffix
	dropletCreateRequest.Name = fmt.Sprintf("%s-%s", properties.NamePrefix, randomSuffix(6))

	// tags to include namespace tags and injected tags
	tags := instance_types.ParseTags(spec)
	_, tags = mergeTags(tags, sliceToMap(properties.Tags), p.namespaceTags) // scope this resource with namespace tags
	dropletCreateRequest.Tags = doTags(mapToStringSlice(tags))

	// SSH key -- the api requires ID but we make it easier so users can use names
	keys := []godo.DropletCreateSSHKey{}
	for _, n := range properties.SSHKeyNames {
		key, err := p.getSshkey(n)
		if err != nil {
			return nil, err
		}
		keys = append(keys, godo.DropletCreateSSHKey{ID: key.ID})
	}
	dropletCreateRequest.SSHKeys = keys

	// CloudInit / UserData
	cloudInit, err := buildCloudInit(dropletCreateRequest.UserData, spec.Init)
	if err != nil {
		return nil, err
	}
	if cloudInit != "" {
		dropletCreateRequest.UserData = cloudInit
	}

	droplet, _, err := p.droplets.Create(context.TODO(), &dropletCreateRequest)
	if err != nil {
		return nil, err
	}
	id := instance.ID(fmt.Sprintf("%d", droplet.ID))
	return &id, nil
}

func (p *plugin) getSshkey(expectedKey string) (godo.Key, error) {
	keys := []godo.Key{}
	islast := false
	page := 0
	for !islast {
		d, resp, err := p.keys.List(context.TODO(), &godo.ListOptions{
			Page: page,
		})
		if err != nil {
			return godo.Key{}, err
		}
		islast = resp.Links.IsLastPage()
		p, err := resp.Links.CurrentPage()
		if err != nil {
			return godo.Key{}, err
		}
		page = p + 1
		keys = append(keys, d...)
	}

	for _, key := range keys {
		if key.Name == expectedKey {
			return key, nil
		}
	}
	log.Warnf("key %s not found", expectedKey)
	return godo.Key{}, nil
}

// Destroy terminates an existing instance.
func (p *plugin) Destroy(instance instance.ID, ctx instance.Context) error {
	id, err := strconv.Atoi(string(instance))
	if err != nil {
		return err
	}

	_, err = p.droplets.Delete(context.TODO(), id)
	return err
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	log.Debugln("describe-instances", tags)

	_, tags = mergeTags(tags, p.namespaceTags)

	droplets, err := p.listDroplets()
	if err != nil {
		return nil, err
	}
	result := []instance.Description{}

	for _, droplet := range droplets {
		instTags := sliceToMap(undoTags(droplet.Tags))
		if hasDifferentTag(tags, instTags) {
			log.Debugf("Skipping %v", droplet.Name)
			continue
		}

		description := instance.Description{
			ID:        instance.ID(fmt.Sprintf("%d", droplet.ID)),
			LogicalID: logicalID(instTags),
			Tags:      instTags,
		}

		if properties {
			if any, err := types.AnyValue(droplet); err == nil {
				description.Properties = any
			} else {
				log.Warningln("error encoding instance properties:", err)
			}
		}

		result = append(result, description)
	}

	return result, nil
}

func logicalID(tags map[string]string) *instance.LogicalID {
	logicalID, present := tags[instance_types.InfrakitLogicalID]
	if present {
		id := instance.LogicalID(logicalID)
		return &id
	}

	return nil
}

func (p *plugin) listDroplets() ([]godo.Droplet, error) {
	droplets := []godo.Droplet{}
	islast := false
	page := 0
	for !islast {
		d, resp, err := p.droplets.List(context.TODO(), &godo.ListOptions{
			Page: page,
		})
		if err != nil {
			return droplets, err
		}
		islast = resp.Links.IsLastPage()
		p, err := resp.Links.CurrentPage()
		if err != nil {
			return droplets, err
		}
		page = p + 1
		droplets = append(droplets, d...)
	}
	return droplets, nil
}

const cloudInitTemplate = `
#cloud-config
{{ $config := dict "runcmd" .lines }}
{{ $config | yamlEncode }}
`

func buildCloudInit(args ...string) (string, error) {
	t, err := template.NewTemplate("str://"+cloudInitTemplate, template.Options{})
	if err != nil {
		return "", err
	}
	lines := []string{}
	for _, l := range args {
		// split the line
		for _, ll := range strings.Split(l, ";") {
			t := strings.Trim(ll, " \t\n")
			if strings.Index(t, "#!") != 0 {
				// exclude shebangs like #!/bin/bash
				lines = append(lines, t)
			}
		}
	}
	return t.Render(map[string]interface{}{"lines": lines})
}

func doTags(tags []string) []string {
	t := []string{}
	for _, tag := range tags {
		t = append(t, strings.Replace(tag, ".", "::", -1))
	}
	return t
}

func undoTags(tags []string) []string {
	t := []string{}
	for _, tag := range tags {
		t = append(t, strings.Replace(tag, "::", ".", -1))
	}
	return t
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
				log.Warnf("Overwriting tag value for key %s", k)
			} else {
				keys = append(keys, k)
			}
			tags[k] = v
		}
	}
	sort.Strings(keys)
	return keys, tags
}

func mapToStringSlice(m map[string]string) []string {
	s := []string{}
	for key, value := range m {
		if value != "" {
			s = append(s, key+":"+value)
		} else {
			s = append(s, key)
		}
	}
	return s
}

func sliceToMap(s []string) map[string]string {
	m := map[string]string{}
	for _, v := range s {
		parts := strings.SplitN(v, ":", 2)
		switch len(parts) {
		case 1:
			m[parts[0]] = ""
		case 2:
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func hasDifferentTag(expected, actual map[string]string) bool {
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

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

// RandomSuffix generate a random instance name suffix of length `n`.
func randomSuffix(n int) string {
	suffix := make([]rune, n)

	for i := range suffix {
		suffix[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	return string(suffix)
}
