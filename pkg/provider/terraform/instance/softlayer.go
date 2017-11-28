package instance

import (
	"fmt"
	"strings"

	"github.com/docker/infrakit/pkg/provider/ibmcloud/client"
	"github.com/softlayer/softlayer-go/datatypes"

	log "github.com/Sirupsen/logrus"
)

const (
	// SoftlayerUsernameEnvVar contains the env var name that the Softlayer terraform
	// provider expects for the Softlayer username
	SoftlayerUsernameEnvVar = "SOFTLAYER_USERNAME"

	// SoftlayerAPIKeyEnvVar contains the env var name that the Softlayer terraform
	// provider expects for the Softlayer API key
	SoftlayerAPIKeyEnvVar = "SOFTLAYER_API_KEY"
)

// mergeLabelsIntoTagSlice combines the tags slice and the labels map into a string slice
// since Softlayer tags are simply strings
func mergeLabelsIntoTagSlice(tags []interface{}, labels map[string]string) []string {
	m := map[string]string{}
	for _, l := range tags {
		line := fmt.Sprintf("%v", l) // conversion using string
		if i := strings.Index(line, ":"); i > 0 {
			key := line[0:i]
			value := ""
			if i+1 < len(line) {
				value = line[i+1:]
			}
			m[key] = value
		} else {
			m[fmt.Sprintf("%v", l)] = ""
		}
	}
	for k, v := range labels {
		m[k] = v
	}

	// now set the final format
	lines := []string{}
	for k, v := range m {
		if v != "" {
			lines = append(lines, fmt.Sprintf("%v:%v", k, v))
		} else {
			lines = append(lines, k)

		}
	}
	return lines
}

// GetIBMCloudVMByTag queries Softlayer for VMs that match all of the given tags. Returns
// the single VM ID that matches or nil if there are no matches.
func GetIBMCloudVMByTag(username, apiKey string, tags []string) (*int, error) {
	c := client.GetClient(username, apiKey)
	mask := "id;hostname;tagReferences"
	vms, err := c.GetVirtualGuests(username, apiKey, &mask)
	if err != nil {
		return nil, err
	}
	return getUniqueVMByTags(vms, tags)
}

// getUniqueVMByTags returns the single VM ID that matches or nil if there are no matches.
func getUniqueVMByTags(vms []datatypes.Virtual_Guest, tags []string) (*int, error) {
	// Filter by tags
	filterVMsByTags(&vms, tags)
	// No match
	if len(vms) == 0 {
		log.Infof("Detected 0 existing VMs with tags: %v", tags)
		return nil, nil
	}
	// Exactly 1 match
	if len(vms) == 1 {
		var name string
		if vms[0].Hostname != nil {
			name = *vms[0].Hostname
		}
		if vms[0].Id == nil {
			return nil, fmt.Errorf("VM '%v' missing ID", name)
		}
		log.Infof("Existing VM %v with ID %v matches tags: %v", name, *vms[0].Id, tags)
		return vms[0].Id, nil
	}
	// More than 1 match
	ids := []int{}
	for _, vm := range vms {
		ids = append(ids, *vm.Id)
	}
	return nil, fmt.Errorf("Only a single VM should match tags, but VMs %v match tags: %v", ids, tags)
}

// filterVMsByTags removes all VM slice entries that do not contain all of the
// given tags
func filterVMsByTags(vms *[]datatypes.Virtual_Guest, tags []string) {
	matches := []datatypes.Virtual_Guest{}
	for _, vm := range *vms {
		allTagsMatch := true
		for _, tag := range tags {
			tagMatch := false
			for _, tagRef := range vm.TagReferences {
				if *tagRef.Tag.Name == tag {
					tagMatch = true
					break
				}
			}
			if !tagMatch {
				allTagsMatch = false
				break
			}
		}
		if allTagsMatch {
			matches = append(matches, vm)
		}
	}
	*vms = matches
}
