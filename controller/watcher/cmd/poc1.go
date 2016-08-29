package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
)

// POC1ConfigFromSWIM parses the swim file to return the config for the controller
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
