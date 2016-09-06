package main

import (
	"encoding/json"
	"errors"
	"fmt"
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

	log.Infoln("Found names in config:", names)
	err = b.registry.Refresh()
	if err != nil {
		log.Warningln("Cannot discover plugins:", err)
		return
	}

	// This is an extra phase in the processing of configs for each controller.
	// In CFN -- this is modeled as dependencies of resources and their ids (which become available as resources
	// are provisioned.  Think of configs as a document that is iteratively evaluated where references are progressively
	// replaced by actual values of provisioned resources or computed values.
	// Here we just pretend and add another round of computation on the configs
	joinToken, err := getWorkerJoinToken(b.docker)
	if err != nil {
		log.Warningln("Problem getting the worker join token. Do nothing. Err=", err)
		return
	}
	log.Infoln("Acquired join token.  Applying to templated config")

	runningContext := map[string]interface{}{
		"JOIN_TOKEN_ARG": fmt.Sprintf("--join-token %s", joinToken),
	}

	// get the configs for all the controllers -- map of controller to config
	changeSet := map[*controller.Controller]interface{}{}

	for _, name := range names {

		// Namespace is more appropriate.  TODO(chungers) - remove name as it is redundant
		controller := b.registry.GetControllerByNamespace(name)
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

			log.Infoln("Another iteration of evaluating the template in the running context. Config=", config, "name=", name)

			// This is where we'd evaluate any templates in the config...
			// One of the config sections for the scaler will have information on setting the UserData for the
			// instance.  The instance uses swarmboot to boot up and swarmboot accepts the join token in its
			// command line arg (see provider/aws/awsbootstrap/create.go).

			// By the the config is parsed from the json as an interface{}.  We check for the type to be map[string]interface{}
			// and then evaluate each string field as if it were a template.
			// There's already a templating engine in the swarm config poc repo for this but here let's hack together
			// a simple template execute.

			// TOOD(chungers) - there are other types. this poc only covers scaler
			poc2Request, ok := config.(poc2scalerRequest)
			log.Infoln("is a poc2 schema scaler request", poc2Request, ok)
			if ok {
				poc2Request.RunInstancesInput = evaluateFieldsAsTemplate(poc2Request.RunInstancesInput, runningContext).(map[string]interface{})
				changeSet[controller] = poc2Request
			} else {
				changeSet[controller] = config
			}
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

		err := controller.Client.Call(controller.Info.DriverType+".Start", config, nil)
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
