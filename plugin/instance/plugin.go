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
	instance_types "github.com/docker/infrakit.digitalocean/plugin/instance/types"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"golang.org/x/oauth2"
)

// Spec is just whatever that can be unmarshalled into a generic JSON map
type Spec map[string]interface{}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

type plugin struct {
	accessToken string
	region      string
}

// NewDOInstancePlugin creates a new DigitalOcean instance plugin for a given region.
func NewDOInstancePlugin(accessToken, region string) instance.Plugin {
	return &plugin{
		accessToken: accessToken,
		region:      region,
	}
}

func (p *plugin) getClient() *godo.Client {
	token := &oauth2.Token{AccessToken: p.accessToken}
	tokenSource := oauth2.StaticTokenSource(token)
	client := oauth2.NewClient(oauth2.NoContext, tokenSource)

	return godo.NewClient(client)
}

// Info returns a vendor specific name and version
func (p *plugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-digitalocean",
			Version: "0.1.0",
		},
		URL: "https://github.com/docker/infrakit.digitalocean",
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
	client := p.getClient()

	client.Droplets.List(context.TODO(), &godo.ListOptions{})
	return nil
}

// Provision creates a new instance based on the spec.
func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {
	properties, err := instance_types.ParseProperties(spec.Properties)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("%s-%s", properties.NamePrefix, randomSuffix(6))

	tags, err := instance_types.ParseTags(spec)
	if err != nil {
		return nil, err
	}
	_, tags = mergeTags(tags, sliceToMap(properties.Tags)) // scope this resource with namespace tags

	client := p.getClient()

	// Create the droplet
	dropletCreateRequest := &godo.DropletCreateRequest{
		Name:   name,
		Region: p.region,
		Size:   properties.Size,
		Image: godo.DropletCreateImage{
			Slug: properties.Image,
		},
		Backups:           properties.Backups,
		IPv6:              properties.IPv6,
		PrivateNetworking: properties.PrivateNetworking,
		Tags:              doTags(mapToStringSlice(tags)),
	}
	droplet, _, err := client.Droplets.Create(context.TODO(), dropletCreateRequest)
	if err != nil {
		return nil, err
	}
	id := instance.ID(fmt.Sprintf("%d", droplet.ID))
	return &id, nil
}

// Destroy terminates an existing instance.
func (p *plugin) Destroy(instance instance.ID) error {
	client := p.getClient()

	id, err := strconv.Atoi(string(instance))
	if err != nil {
		return err
	}

	_, err = client.Droplets.Delete(context.TODO(), id)
	return err
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	log.Debugln("describe-instances", tags)

	client := p.getClient()
	droplets, _, err := client.Droplets.List(context.TODO(), &godo.ListOptions{
		// FIXME(vdemeester) handle pagination (using resp.Pages)
		PerPage: 100,
	})
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
			ID:   instance.ID(fmt.Sprintf("%d", droplet.ID)),
			Tags: instTags,
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
	log.Debugf("expected: %v", expected)
	log.Debugf("actual: %v", actual)
	for k, v := range expected {
		if actual[k] != v {
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
