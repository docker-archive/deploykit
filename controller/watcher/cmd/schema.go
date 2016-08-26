package main

import (
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
)

type poc2scalerRequest struct {
	Group string `json:"group"`
	Count int    `json:"count"`
}

type poc2schema struct {
	Driver string
	Groups map[string]struct {
		Config poc2scalerRequest
		Size   int
		Type   string
	}
}

// POC2ImageToConnectionString returns the connection string used by the client to
// configure the driver.
// TODO(chungers) -- Need to integrate this with Plugin Discovery in the Engine.
func POC2ImageToConnectionString(controllerImage string) string {
	switch controllerImage {
	case "libmachete/scaler":
		return "localhost:9090" // TODO(chungers) - won't work if watcher runs in container.
	}
	return ""
}

// POC2GetImagesFromSWIM parses the swim file to determine the images to restart
// TODO(chungers) -- Need a mapping of driver to image
func POC2ImagesFromSWIM(buff []byte) ([]string, error) {
	swim := new(poc2schema)
	err := json.Unmarshal(buff, swim)
	if err != nil {
		return nil, err
	}
	images := []string{}
	switch swim.Driver {
	case "aws":
		images = append(images, "libmachete/scaler")
	}
	return images, nil
}

// POC2ConfigFromSWIM
func POC2ConfigFromSWIM(buff []byte, controllerImage string) (interface{}, error) {
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
func POC1ConfigFromSWIM(buff []byte, controllerImage string) (interface{}, error) {
	// Get the controllers
	swim := map[string]interface{}{}
	if err := json.Unmarshal(buff, &swim); err != nil {
		return nil, err
	}
	for _, block := range swim {
		if driver := getMap(block, "driver"); driver != nil {
			if driver["image"].(string) == controllerImage {
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

// POC1ImagesFromSWIM parses the swim file and determine a list of containers by image
func POC1ImagesFromSWIM(buff []byte) ([]string, error) {
	// Get the controllers
	swim := map[string]interface{}{}
	if err := json.Unmarshal(buff, &swim); err != nil {
		return nil, err
	}
	images := []string{}
	for resource, block := range swim {
		if driver := getMap(block, "driver"); driver != nil {
			if image, ok := driver["image"]; ok {
				if i, ok := image.(string); ok {
					images = append(images, i)
					log.Infoln("controller image", i, "found for resource", resource)
				}
			}
		}
	}
	return images, nil
}
