package instance

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/packethost/packngo"
)

var log = logutil.New("module", "plugin/instance/packet")

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// Properties is the input in the `Properties` field of the config yml
type Properties struct {
	*packngo.DeviceCreateRequest `yaml:",inline"`

	// HostnamePrefix
	HostnamePrefix string `json:"hostname_prefix,omitempty"`

	// Checksum
	Checksum string `json:"checksum,omitempty"`
}

type plugin struct {
	projectID     string
	client        *packngo.Client
	namespaceTags map[string]string
}

// NewPlugin creates a new DigitalOcean instance plugin for a given region.
func NewPlugin(projectID, apiKey string, namespace map[string]string) instance.Plugin {
	return &plugin{
		projectID:     projectID, // required for list()
		client:        packngo.NewClient("", apiKey, nil),
		namespaceTags: namespace,
	}
}

// Info returns a vendor specific name and version
func (p *plugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-packetnet",
			Version: "0.1.0",
		},
		URL: "https://github.com/docker/infrakit",
	}
}

// Validate performs local validation on a provision request.
func (p *plugin) Validate(req *types.Any) error {
	properties := &Properties{}
	return req.Decode(&properties)
}

// Label labels the instance
func (p *plugin) Label(instance instance.ID, labels map[string]string) error {
	// You can't tag things after they are created
	return nil
}

// Provision creates a new instance based on the spec.
func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {

	properties := &Properties{}
	if err := spec.Properties.Decode(&properties); err != nil {
		return nil, err
	}

	deviceCreateRequest := *properties.DeviceCreateRequest

	// Some computed overrides:
	if deviceCreateRequest.ProjectID == "" {
		deviceCreateRequest.ProjectID = p.projectID
	}

	// the name must be given suffix
	deviceCreateRequest.HostName = fmt.Sprintf("%s-%s", properties.HostnamePrefix, randomSuffix(6))

	// tags to include namespace tags and injected tags
	tags := parseTags(spec)
	_, tags = mergeTags(tags, p.namespaceTags) // scope this resource with namespace tags
	deviceCreateRequest.Tags = doTags(mapToStringSlice(tags))

	// CloudInit / UserData
	if spec.Init != "" {
		deviceCreateRequest.UserData = strings.Join([]string{
			deviceCreateRequest.UserData,
			spec.Init,
		}, "\n")
	}

	log.Debug("creating", "request", deviceCreateRequest)

	device, _, err := p.client.Devices.Create(&deviceCreateRequest)
	if err != nil {
		return nil, err
	}
	id := instance.ID(fmt.Sprintf("%s", device.ID))
	return &id, nil
}

// Destroy terminates an existing instance.
func (p *plugin) Destroy(instance instance.ID, ctx instance.Context) error {
	_, err := p.client.Devices.Delete(string(instance))
	return err
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	log.Debug("describe-instances", "tags", tags)

	_, tags = mergeTags(tags, p.namespaceTags)

	devices, _, err := p.client.Devices.List(p.projectID)
	if err != nil {
		return nil, err
	}
	result := []instance.Description{}

	for _, device := range devices {
		instTags := sliceToMap(undoTags(device.Tags))
		if hasDifferentTag(tags, instTags) {
			log.Debug("Skipping", "id", device.ID, "hostname", device.Hostname)
			continue
		}

		description := instance.Description{
			ID:   instance.ID(fmt.Sprintf("%s", device.ID)),
			Tags: instTags,
		}

		if properties {
			if any, err := types.AnyValue(device); err == nil {
				description.Properties = any
			} else {
				log.Warn("error encoding instance properties", "err", err)
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

// parseTags returns a key/value map from the instance specification.
func parseTags(spec instance.Spec) map[string]string {
	tags := make(map[string]string)

	for k, v := range spec.Tags {
		tags[k] = v
	}

	// Do stuff with proprerties here

	if spec.LogicalID != nil {
		tags["infrakit.logicalID"] = string(*spec.LogicalID)
	}

	return tags
}
