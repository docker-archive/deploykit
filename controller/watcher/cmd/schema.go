package main

import (
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
)

type poc2scalerRequest struct {
	Group             string                 `json:"group"`
	Count             int                    `json:"count"`
	RunInstancesInput map[string]interface{} `json:"run_instances_input"`
}

type poc2schema struct {
	Driver string
	Groups map[string]struct {
		Config poc2scalerRequest
		Size   int
		Type   string
	}
}

// POC2ControllerNamesFromSWIM parses the swim file to determine the images to restart
// TODO(chungers) -- Need a mapping of driver to image
func POC2ControllerNamesFromSWIM(buff []byte) ([]string, error) {
	swim := new(poc2schema)
	err := json.Unmarshal(buff, swim)
	if err != nil {
		return nil, err
	}
	images := []string{
		swim.Driver,
	}
	return images, nil
}

// POC2ConfigFromSWIM
func POC2ConfigFromSWIM(buff []byte, controllerNamespace string) (interface{}, error) {

	// This schema is a simplified schema that has only one driver. For the config,
	// we have to somehow identify the Workers....  the schema doesn't support that
	// so we are just looking for a group that IS NOT managers.

	swim := new(poc2schema)
	err := json.Unmarshal(buff, swim)
	if err != nil {
		return nil, err
	}
	if swim.Groups == nil {
		return nil, errors.New("no-groups")
	}
	group, exists := swim.Groups["Workers"]
	if !exists {
		return nil, errors.New("missing Workers -- yes it's hardcoded")
	}
	if group.Type == "manager" {
		return nil, errors.New("bad data -- this shouldn't be manager")
	}

	group.Config.Count = group.Size
	return group.Config, nil
}

// POC1ConfigFromSWIM
func POC1ConfigFromSWIM(buff []byte, controllerNamespace string) (interface{}, error) {
	// Get the controllers
	swim := map[string]interface{}{}
	if err := json.Unmarshal(buff, &swim); err != nil {
		return nil, err
	}
	for _, block := range swim {
		if driver := getMap(block, "driver"); driver != nil {
			if driver["image"].(string) == controllerNamespace {
				var count interface{}
				if m, ok := block.(map[string]interface{}); ok {
					count = m["count"]
				}

				if properties, ok := driver["properties"]; ok {
					if c, ok := properties.(map[string]interface{}); ok {
						c["count"] = count
						return c, nil
					}
				}
			}
		}
	}
	return nil, nil
}

// POC1ControllerNamesFromSWIM parses the swim file and determine a list of containers by image
func POC1ControllerNamesFromSWIM(buff []byte) ([]string, error) {
	// Get the controllers
	swim := map[string]interface{}{}
	if err := json.Unmarshal(buff, &swim); err != nil {
		return nil, err
	}
	names := []string{}
	for resource, block := range swim {
		if driver := getMap(block, "driver"); driver != nil {
			if name, ok := driver["name"]; ok {
				if i, ok := name.(string); ok {
					names = append(names, i)
					log.Infoln("controller name", i, "found for resource", resource)
				}
			}
		}
	}
	return names, nil
}
