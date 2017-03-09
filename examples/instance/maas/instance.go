package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	maas "github.com/juju/gomaasapi"
	"net/url"
)

// NewMaasPlugin creates an instance plugin for MaaS.
func NewMaasPlugin(dir string, key string, url string, version string) instance.Plugin {
	var err error
	var authClient *maas.Client
	if key != "" {
		authClient, err = maas.NewAuthenticatedClient(url, key, version)
	} else {
		authClient, err = maas.NewAnonymousClient(url, version)
	}
	if err != nil {
		return nil
	}
	maasobj := maas.NewMAAS(*authClient)
	return &maasPlugin{MaasfilesDir: dir, MaasObj: maasobj}
}

type maasPlugin struct {
	MaasfilesDir string
	MaasObj      *maas.MAASObject
}

// Validate performs local validation on a provision request.
func (m maasPlugin) Validate(req *types.Any) error {
	return nil
}

func (m maasPlugin) convertSpecToMaasParam(spec map[string]interface{}) url.Values {
	param := url.Values{}
	return param
}

func (m maasPlugin) checkDuplicate(systemID string) (bool, error) {
	files, err := ioutil.ReadDir(m.MaasfilesDir)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		machineDir := path.Join(m.MaasfilesDir, file.Name())
		hID, err := ioutil.ReadFile(path.Join(machineDir, "MachineID"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, err
		}
		if systemID == string(hID) {
			return true, nil
		}
	}
	return false, nil
}

// Provision creates a new instance.
func (m maasPlugin) Provision(spec instance.Spec) (*instance.ID, error) {

	var properties map[string]interface{}

	if spec.Properties != nil {
		if err := spec.Properties.Decode(&properties); err != nil {
			return nil, fmt.Errorf("Invalid instance properties: %s", err)
		}
	}
	nodeListing := m.MaasObj.GetSubObject("nodes")
	jsonResponse, err := nodeListing.CallPost("acquire", m.convertSpecToMaasParam(properties))
	if err != nil {
		return nil, err
	}
	acquiredNode, err := jsonResponse.GetMAASObject()
	if err != nil {
		return nil, err
	}
	listNodeObjects, err := nodeListing.CallGet("list", url.Values{})
	if err != nil {
		return nil, err
	}
	listNodes, err := listNodeObjects.GetArray()
	if err != nil {
		return nil, err
	}
	notDuplicate := false
	for range listNodes {
		systemID, err := acquiredNode.GetField("system_id")
		if err != nil {
			return nil, err
		}
		isdup, err := m.checkDuplicate(systemID)
		if err != nil {
			return nil, err
		}
		if isdup {
			jsonResponse, err = nodeListing.CallPost("acquire", m.convertSpecToMaasParam(properties))
			if err != nil {
				return nil, err
			}
			acquiredNode, err = jsonResponse.GetMAASObject()
			if err != nil {
				return nil, err
			}
		} else {
			notDuplicate = true
			break
		}
	}
	if !notDuplicate {
		return nil, errors.New("Failed to aquire node")
	}
	isAcquired := make(chan bool)
	go func() {
		for state, err := acquiredNode.GetField("substatus_name"); state != "Allocated" && err == nil; state, err = acquiredNode.GetField("substatus_name") {
			time.Sleep(100)
		}
		isAcquired <- true
		return
	}()
	<-isAcquired
	params := url.Values{}
	if _, err = acquiredNode.CallPost("start", params); err != nil {
		return nil, err
	}
	systemID, err := acquiredNode.GetField("system_id")
	if err != nil {
		return nil, err
	}
	id := instance.ID(systemID)
	machineDir, err := ioutil.TempDir(m.MaasfilesDir, "infrakit-")
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(path.Join(machineDir, "MachineID"), []byte(systemID), 0755); err != nil {
		return nil, err
	}
	tagData, err := types.AnyValue(spec.Tags)
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(path.Join(machineDir, "tags"), tagData.Bytes(), 0666); err != nil {
		return nil, err
	}
	if spec.LogicalID != nil {
		if err := ioutil.WriteFile(path.Join(machineDir, "ip"), []byte(*spec.LogicalID), 0666); err != nil {
			return nil, err
		}
	}
	return &id, nil
}

// Label labels the instance
func (m maasPlugin) Label(id instance.ID, labels map[string]string) error {
	files, err := ioutil.ReadDir(m.MaasfilesDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		machineDir := path.Join(m.MaasfilesDir, file.Name())
		systemID, err := ioutil.ReadFile(path.Join(machineDir, "MachineID"))
		if err != nil {
			return err
		}
		if id == instance.ID(systemID) {

			tagFile := path.Join(machineDir, "tags")
			buff, err := ioutil.ReadFile(tagFile)
			if err != nil {
				return err
			}

			tags := map[string]string{}
			err = types.AnyBytes(buff).Decode(&tags)
			if err != nil {
				return err
			}

			for k, v := range labels {
				tags[k] = v
			}

			encoded, err := types.AnyValue(tags)
			if err != nil {
				return err
			}
			return ioutil.WriteFile(tagFile, encoded.Bytes(), 0666)
		}
	}
	return nil
}

// Destroy terminates an existing instance.
func (m maasPlugin) Destroy(id instance.ID) error {
	fmt.Println("Destroying ", id)
	nodeListing := m.MaasObj.GetSubObject("nodes")
	listNodeObjects, err := nodeListing.CallGet("list", url.Values{})
	if err != nil {
		return err
	}
	listNodes, err := listNodeObjects.GetArray()
	for _, nodeObj := range listNodes {
		node, err := nodeObj.GetMAASObject()
		systemID, err := node.GetField("system_id")
		if systemID == string(id) {
			if state, _ := node.GetField("substatus_name"); state == "Deploying" {
				params := url.Values{}
				if _, err = node.CallPost("abort_operation", params); err != nil {
					return err
				}
			}
			params := url.Values{}
			if _, err = node.CallPost("release", params); err != nil {
				return err
			}
		}
	}
	files, err := ioutil.ReadDir(m.MaasfilesDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		machineDir := path.Join(m.MaasfilesDir, file.Name())
		systemID, err := ioutil.ReadFile(path.Join(machineDir, "MachineID"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		} else if id == instance.ID(systemID) {
			if err := os.RemoveAll(machineDir); err != nil {
				return err
			}
		}
	}
	return nil
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (m maasPlugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	files, err := ioutil.ReadDir(m.MaasfilesDir)
	if err != nil {
		return nil, err
	}
	descriptions := []instance.Description{}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		machineDir := path.Join(m.MaasfilesDir, file.Name())
		tagData, err := ioutil.ReadFile(path.Join(machineDir, "tags"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		machineTags := map[string]string{}
		if err := types.AnyBytes(tagData).Decode(&machineTags); err != nil {
			return nil, err
		}
		allMatched := true
		for k, v := range tags {
			value, exists := machineTags[k]
			if !exists || v != value {
				allMatched = false
				break
			}
		}
		if allMatched {
			systemID, err := ioutil.ReadFile(path.Join(machineDir, "MachineID"))
			if err == nil {
			} else {
				if !os.IsNotExist(err) {
					return nil, err
				}
			}
			descriptions = append(descriptions, instance.Description{
				ID:   instance.ID(systemID),
				Tags: machineTags,
			})
		}
	}
	return descriptions, nil
}
