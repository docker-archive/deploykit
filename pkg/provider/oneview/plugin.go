package oneview

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"

	"github.com/HewlettPackard/oneview-golang/ov"
)

var log = logutil.New("module", "provider/oneview")

// Options capture the config parameters required to create the plugin
type Options struct {
	OVUrl    string
	OVUser   string
	OVPass   string
	OVCookie string
	OVApi    int
}

//miniFSM for managing the provisioning -> provisioned state
type provisioningFSM struct {
	countdown    int64             // ideally will be a counter of minutes / seconds
	tags         map[string]string // tags that will be passed back per a describe function
	instanceName string            // name that we will use as a lookup to the actual backend that is privisioning
}

// Spec is just whatever that can be unmarshalled into a generic JSON map
type Spec map[string]interface{}

// This contains the the details for the oneview instance
type plugin struct {
	fsm    []provisioningFSM
	client ov.OVClient
}

var mux sync.Mutex

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// NewOneViewInstancePlugin will take the cmdline/env configuration
func NewOneViewInstancePlugin(ovOptions Options) instance.Plugin {

	// Define client from params
	var client ov.OVClient
	client.Endpoint = ovOptions.OVUrl
	client.User = ovOptions.OVUser
	client.Password = ovOptions.OVPass
	client.APIVersion = ovOptions.OVApi

	// Attempt to log in to HPE OneView, if a cookie is passed then just re-auth, or login with credentials
	session, err := client.SessionLogin()

	// More verbose erroring might be needed i.e. https not http (also a protocl prefix)
	// Exit with an error if we can't connect to HPE OneView
	if err != nil {
		log.Crit("Error Logging into HPE OneView")
		os.Exit(-1)
	}

	log.Debug("Succesfully logged in with", "sessionID", session.ID)

	return &plugin{
		client: client,
	}
}

// Info returns a vendor specific name and version
func (p *plugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-oneview",
			Version: "0.6.0",
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
	log.Debug("validate", req.String())

	spec := Spec{}
	if err := req.Decode(&spec); err != nil {
		return err
	}

	log.Debug("Validated:", spec)
	return nil
}

// Provision creates a new instance based on the spec.
func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {

	var properties map[string]interface{}

	if spec.Properties != nil {
		if err := spec.Properties.Decode(&properties); err != nil {
			return nil, fmt.Errorf("Invalid instance properties: %s", err)
		}
	}

	instanceName := instance.ID(fmt.Sprintf("InfraKit-%d", rand.Int63()))

	// Task isn't backgrounded with a goroutine as that caused numerous issues with teh web requests
	var template string

	if val, ok := properties["TemplateName"]; ok {
		// Assign the string value
		template = val.(string)
	} else {
		log.Error("InfraKit Tag TemplateName has been left blank")
	}

	profileTemplate, err := p.client.GetProfileTemplateByName(template)
	if err != nil {
		log.Warn("Error returning list of profiles %v", err)
	}
	availHW, err := p.client.GetAvailableHardware(profileTemplate.ServerHardwareTypeURI, profileTemplate.EnclosureGroupURI)
	if err != nil {
		log.Warn("Error returning list of profiles %v", err)
	}

	// Build a custom Description to allow InfraKit to identify new Instances
	profileTemplate.Description = spec.Tags[group.GroupTag] + "|" + spec.Tags[group.ConfigSHATag]

	err = p.createProfileFromTemplate(string(instanceName), profileTemplate, availHW, spec)
	if err != nil {
		log.Error("%v", err)
	}

	if spec.Tags != nil {
		log.Info("Adding %s to Group %v", string(instanceName), spec.Tags[group.GroupTag])
	}

	var newInstance provisioningFSM
	newInstance.instanceName = string(instanceName)
	newInstance.countdown = 5 // FIXED 10 minute timeout (TODO)

	// duplicate the tags for the instance
	newInstance.tags = make(map[string]string)
	for k, v := range spec.Tags {
		newInstance.tags[k] = v
	}
	newInstance.tags["infrakit.state"] = "Provisioning"
	p.fsm = append(p.fsm, newInstance)
	log.Debug("New Instance added to state, count: %d", len(p.fsm))

	return &instanceName, nil
}

// Label labels the instance
func (p *plugin) Label(instance instance.ID, labels map[string]string) error {
	return fmt.Errorf("HPE OneView label updates are not implemented yet")
}

// Destroy terminates an existing instance.
func (p *plugin) Destroy(instance instance.ID, context instance.Context) error {
	log.Info("Currently running %s on instance: %v", context, instance)
	return p.client.DeleteProfile(string(instance))
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
// TODO - need to define the fitlering of tags => AND or OR of matches?
func (p *plugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	log.Debug("describe-instances", tags)
	results := []instance.Description{}

	// Static search path for Profiles that are pre-fixed with the InfraKit- tag
	instances, err := p.client.GetProfiles("name matches 'InfraKit-%'", "")
	if err != nil {
		log.Warn("Error returning list of profiles %v", err)
	}

	log.Debug("Found %d Profiles", instances.Total)

	// Duplicate original tags
	for _, profile := range instances.Members {
		instanceTags := make(map[string]string)
		for k, v := range tags {
			instanceTags[k] = v
		}
		// Split the description field (single line)
		tagSlice := strings.Split(profile.Description, "|")
		// If it exists, grab the group name
		if len(tagSlice) > 0 {
			instanceTags[group.GroupTag] = tagSlice[0]
		}
		// If it exists, grab the sha
		if len(tagSlice) > 1 {
			instanceTags[group.ConfigSHATag] = tagSlice[1]
		}

		// We're only wanting to return instances from a specific group
		if val, ok := tags[group.GroupTag]; ok {
			// Check that we can get the group from the profile
			if len(tagSlice) > 0 {
				// Does this group match, if so add it to the results
				if val == tagSlice[0] {
					results = append(results, instance.Description{
						ID:        instance.ID(profile.Name),
						LogicalID: nil,
						Tags:      instanceTags,
					})
				}
			}
		} else {
			// Return all
			results = append(results, instance.Description{
				ID:        instance.ID(profile.Name),
				LogicalID: nil,
				Tags:      instanceTags,
			})
		}
	}

	log.Debug("Modifying provisining state count:  %d", len(p.fsm))

	// DIFF what the endpoint is saying as reported versus what we have in the FSM
	var updatedFSM []provisioningFSM
	for _, unprovisionedInstance := range p.fsm {
		var provisioned bool

		for _, provisionedInstance := range results {

			if string(provisionedInstance.ID) == unprovisionedInstance.instanceName {
				provisioned = true
				// instance has been provisioned so break from loop
				break
			} else {
				provisioned = false
			}
		}
		if provisioned == false && unprovisionedInstance.countdown != 0 && unprovisionedInstance.tags[group.GroupTag] == tags[group.GroupTag] {
			unprovisionedInstance.countdown--
			updatedFSM = append(updatedFSM, unprovisionedInstance)
		}
	}

	p.fsm = make([]provisioningFSM, len(updatedFSM))
	copy(p.fsm, updatedFSM)

	log.Debug("Updated provisining state count: %d", len(p.fsm))
	for _, unprovisionedInstances := range p.fsm {
		results = append(results, instance.Description{
			ID:        instance.ID(unprovisionedInstances.instanceName),
			LogicalID: nil,
			Tags:      unprovisionedInstances.tags,
		})
	}
	return results, nil
}

// create profile from template
func (p *plugin) createProfileFromTemplate(name string, template ov.ServerProfile, blade ov.ServerHardware, spec instance.Spec) error {
	log.Debug("TEMPLATE : %+v\n", template)
	var (
		newTemplate ov.ServerProfile
		err         error
	)

	if p.client.IsProfileTemplates() {
		log.Debug("getting profile by URI %+v, v2", template.URI)
		newTemplate, err = p.client.GetProfileByURI(template.URI)
		if err != nil {
			return err
		}
		newTemplate.Type = "ServerProfileV5"
		newTemplate.ServerProfileTemplateURI = template.URI // create relationship
		log.Debug("new_template -> %+v", newTemplate)
	} else {
		log.Debug("get new_template from clone, v1")
		newTemplate = template.Clone()
	}
	newTemplate.ServerHardwareURI = blade.URI
	// HPE OneView doesn't carry any concept of tags, we place all details needed in the Description field
	newTemplate.Description = spec.Tags[group.GroupTag] + "|" + spec.Tags[group.ConfigSHATag]
	newTemplate.Name = name

	t, err := p.client.SubmitNewProfile(newTemplate)
	if err != nil {
		return err
	}
	// TODO: This prints out a lot of verbose text using a different logger, so will need changing.
	err = t.Wait()
	if err != nil {
		return err
	}

	return nil
}
