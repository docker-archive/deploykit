package main

import (
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/controller"
	"github.com/docker/libmachete/controller/watcher"
)

type poc2scalerRequest struct {
	Group             string                 `json:"group"`
	Count             int                    `json:"count"`
	RunInstancesInput map[string]interface{} `json:"run_instances_input"`
}

type poc2schema struct {
	ClusterName string
	Driver      string
	Groups      map[string]struct {
		Config poc2scalerRequest
		Size   int
		Type   string // manager | worker
	}
}

// POC2Reactor reacts to change in the swim file.
func (b *backend) POC2Reactor(buff []byte) {
	log.Infoln("Change detected. Restarting controllers")
	names, err := POC2ControllerNamesFromSWIM(buff)
	if err != nil {
		log.Warningln("Cannot parse input.", err)
		return
	}

	// get the configs for all the controllers -- map of controller to config
	changeSet := map[*controller.Controller]interface{}{}

	for _, name := range names {

		controller := b.registry.GetControllerByName(name)
		if controller == nil {
			log.Warningln("No controller found for name=", name, "Do nothing.")
			return
		}

		config, err := POC2ConfigFromSWIM(buff, controller.Info.Namespace)
		if err != nil {
			log.Warningln("Error while locating configuration of controller", controller.Info.Name)
			return
		}

		// config can be null...
		// TODO(chungers) -- think about this case... no config we assume no change / no need to restart.

		if config != nil {
			changeSet[controller] = config
		}
	}

	// Now run the changes
	// Note there's no specific ordering.  If we are smart we could build dependency into the swim like CFN ;)

	for controller := range changeSet {

		log.Infoln("Restarting controller", controller.Info.Name)
		restart := watcher.Restart(b.docker, controller.Info.Image)

		err := restart.Run()
		if err != nil {
			log.Warningln("Unable to restart controller", controller.Info.Name)

			// TODO(chungers) -- Do we fail here???  If not all controllers can come back up
			// then we cannot to any updates and cannot maintain state either...

			continue
		}
	}

	// At this point all controllers are running in a latent state.
	// reconfigure all the controllers

	for controller, config := range changeSet {

		log.Infoln("Configuring controller", controller.Info.Name, "with config", config)

		err := controller.Client.Call(controller.Info.DriverType+".Start", config)
		if err != nil {
			// BAD NEWS -- here we cannot get consistency now since one of the controller cannot
			// be updated.  Should we punt -- roll back is impossible at the moment
			log.Warningln("Failed to reconfigure controller", controller.Info.Name, "err=", err)
		}

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

// POC2ConfigFromSWIM returns the config for the controller, given its namespace
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
