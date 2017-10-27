package instance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
	apitypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"golang.org/x/net/context"
)

const (
	userdataDirname = "/var/lib"
)

var userdataBasename = "userdata"

type dockerInstancePlugin struct {
	client         client.Client
	ctx            context.Context
	namespaceTags  map[string]string
	metadataPlugin metadata.Plugin
}

type properties struct {
	Host     string
	Retries  int
	Instance *types.Any
}

// NewInstancePlugin creates a new plugin that creates instances on the Docker host
func NewInstancePlugin(client *client.Client, namespaceTags map[string]string) instance.Plugin {
	d := dockerInstancePlugin{client: *client, ctx: context.Background(), namespaceTags: namespaceTags}
	return &d
}

func (p dockerInstancePlugin) tagInstance(
	id *(instance.ID),
	systemTags map[string]string,
	userTags map[string]string) error {
	// todo
	return nil
}

// CreateInstanceRequest is the concrete provision request type.
type CreateInstanceRequest struct {
	Tags               map[string]string
	Config             *container.Config
	HostConfig         *container.HostConfig
	NetworkAttachments []*apitypes.NetworkResource
}

// VendorInfo returns a vendor specific name and version
func (p dockerInstancePlugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-docker",
			Version: "0.4.1",
		},
		URL: "https://github.com/docker/infrakit.docker",
	}
}

// ExampleProperties returns the properties / config of this plugin
func (p dockerInstancePlugin) ExampleProperties() *types.Any {
	config := container.Config{
		Image: "docker/dind",
		Env: []string{
			"var1=value1",
			"var2=value2",
		},
	}
	hostConfig := container.HostConfig{}
	example := CreateInstanceRequest{
		Tags: map[string]string{
			"tag1": "value1",
			"tag2": "value2",
		},
		Config:     &config,
		HostConfig: &hostConfig,
	}

	any, err := types.AnyValue(example)
	if err != nil {
		panic(err)
	}
	return any
}

// Validate performs local checks to determine if the request is valid
func (p dockerInstancePlugin) Validate(req *types.Any) error {
	return nil
}

// Label implements labeling the instances
func (p dockerInstancePlugin) Label(id instance.ID, labels map[string]string) error {
	return fmt.Errorf("Docker container label updates are not implemented yet")
}

// mergeTags merges multiple maps of tags, implementing 'last write wins' for colliding keys.
//
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

// Provision creates a new instance
func (p dockerInstancePlugin) Provision(spec instance.Spec) (*instance.ID, error) {

	if spec.Properties == nil {
		return nil, errors.New("Properties must be set")
	}

	request := CreateInstanceRequest{}
	err := json.Unmarshal(*spec.Properties, &request)
	if err != nil {
		return nil, fmt.Errorf("invalid input formatting: %s", err)
	}
	if request.Config == nil {
		return nil, errors.New("Config should be set")
	}
	// merge tags
	_, allTags := mergeTags(spec.Tags, request.Tags)
	request.Config.Labels = allTags

	cli := p.client
	ctx := context.Background()
	image := request.Config.Image
	if image == "" {
		return nil, errors.New("A Docker image should be specified")
	}
	reader, err := cli.ImagePull(ctx, image, apitypes.ImagePullOptions{})
	if err != nil {
		// don't exit, if no access to the registry we may still want to run the container
		log.Warnf("ImagePull failed: %f", err)
	} else {
		data := make([]byte, 1000, 1000)
		for {
			_, err := reader.Read(data)
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				return nil, err
			}
		}
	}
	containerName := ""
	if spec.LogicalID != nil {
		containerName = string(*spec.LogicalID)
	}
	log.Debugf("Creating a new container (%s)", containerName)
	r, err := cli.ContainerCreate(ctx, request.Config, request.HostConfig, nil, containerName)
	if err != nil {
		return nil, err
	}

	if r.ID == "" {
		return nil, errors.New("Unexpected Docker API response")
	}
	id := (instance.ID)(r.ID[0:12])
	log.Debugf("Container ID = %s", r.ID[0:12])

	// If a network attachment is specified in the config
	// attach the container to these networks
	// if the network do not exist, first create it
	for _, networkAttachment := range request.NetworkAttachments {
		if networkAttachment.Name == "" {
			_ = p.Destroy(id, instance.Context{})
			return nil, errors.New("NetworkAttachment should have a name")
		}
		filter := filters.NewArgs()
		filter.Add("name", networkAttachment.Name)
		if networkAttachment.Driver != "" {
			filter.Add("driver", networkAttachment.Driver)
		}
		log.Debugf("Attaching container to network %s", networkAttachment.Name)
		networkListOptions := apitypes.NetworkListOptions{Filters: filter}
		networkResources, err := cli.NetworkList(ctx, networkListOptions)
		if err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}
		networkID := ""
		if len(networkResources) == 0 {
			// create the network
			log.Debug("Creating the network")
			networkCreateOptions := apitypes.NetworkCreate{Driver: networkAttachment.Driver, Attachable: true, CheckDuplicate: true}
			networkResponse, err := cli.NetworkCreate(ctx, networkAttachment.Name, networkCreateOptions)
			if err != nil {
				_ = p.Destroy(id, instance.Context{})
				return nil, err
			}
			networkID = networkResponse.ID
		} else if len(networkResources) == 1 {
			networkID = networkResources[0].ID
		} else {
			_ = p.Destroy(id, instance.Context{})
			return nil, fmt.Errorf("too many (%d) networks found with name %s", len(networkResources), networkAttachment.Name)
		}

		endpointSettings := network.EndpointSettings{}
		err = cli.NetworkConnect(ctx, networkID, r.ID, &endpointSettings)
		if err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}
	}

	// copy the userdata file in the container
	if spec.Init != "" {
		tmpfile, err := ioutil.TempFile("/tmp", "infrakit")
		userdataBasename = path.Base(tmpfile.Name())
		if err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}

		defer os.Remove(tmpfile.Name()) // clean up

		if _, err := tmpfile.Write([]byte(spec.Init)); err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}
		if err := tmpfile.Close(); err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}
		srcInfo, err := archive.CopyInfoSourcePath(tmpfile.Name(), false)
		if err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}

		srcArchive, err := archive.TarResource(srcInfo)
		if err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}
		defer srcArchive.Close()
		dstDir, preparedArchive, err := archive.PrepareArchiveCopy(srcArchive, srcInfo, archive.CopyInfo{Path: fmt.Sprintf("%s/%s", userdataDirname, userdataBasename)})
		if err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}
		defer preparedArchive.Close()

		if err := cli.CopyToContainer(ctx, r.ID, dstDir, preparedArchive, apitypes.CopyToContainerOptions{}); err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}
		containerPathStat, err := cli.ContainerStatPath(ctx, r.ID, fmt.Sprintf("%s/%s", userdataDirname, userdataBasename))
		if err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}
		if containerPathStat.Size != int64(len(spec.Init)) {
			_ = p.Destroy(id, instance.Context{})
			return nil, fmt.Errorf("userdata has not been properly copied in the container, filename = %s, file size = %d", containerPathStat.Name, containerPathStat.Size)
		}
	}

	if err = cli.ContainerStart(ctx, r.ID, apitypes.ContainerStartOptions{}); err != nil {
		_ = p.Destroy(id, instance.Context{})
		return nil, err
	}

	if spec.Init != "" {
		log.Debug("exec the init script")
		execConfig := apitypes.ExecConfig{Cmd: []string{"/bin/sh", fmt.Sprintf("%s/%s", userdataDirname, userdataBasename)}}
		idResponse, err := cli.ContainerExecCreate(ctx, r.ID, execConfig)
		if err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}
		if err = cli.ContainerExecStart(ctx, idResponse.ID, apitypes.ExecStartCheck{}); err != nil {
			_ = p.Destroy(id, instance.Context{})
			return nil, err
		}
		exitCode := 0
		keepLooping := true
		for keepLooping {
			execInspect, err := cli.ContainerExecInspect(ctx, idResponse.ID)
			if err != nil {
				log.Warningf("failed to inspect the execution, cmd was: %s", execConfig.Cmd)
				_ = p.Destroy(id, instance.Context{})
				return nil, err
			}
			exitCode = execInspect.ExitCode
			keepLooping = execInspect.Running
		}
		if exitCode != 0 {
			log.Warningf("failed to exec init script for container %d", r.ID[0:12])
			rc, err := cli.ContainerLogs(ctx, r.ID, apitypes.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
			if err != nil {
				log.Warning(err)
			} else {
				buf := new(bytes.Buffer)
				buf.ReadFrom(rc)
				log.Warning(buf.String())
			}
			_ = p.Destroy(id, instance.Context{})
			return nil, fmt.Errorf("init script failed with code %d for container %s", exitCode, r.ID[0:12])
		}
		log.Debug("Init script succesfully executed")
	}
	return &id, nil
}

// Destroy terminates an existing instance
func (p dockerInstancePlugin) Destroy(id instance.ID, dc instance.Context) error {
	options := apitypes.ContainerRemoveOptions{Force: true, RemoveVolumes: true, RemoveLinks: false}
	cli := p.client
	ctx := context.Background()
	log.Debugf("Destroying container ID %s", string(id))
	return cli.ContainerRemove(ctx, string(id), options)
}

func describeGroupRequest(namespaceTags, tags map[string]string) *apitypes.ContainerListOptions {

	filter := filters.NewArgs()
	filter.Add("status", "created")
	filter.Add("status", "running")

	keys, allTags := mergeTags(tags, namespaceTags)

	for _, key := range keys {
		filter.Add("label", fmt.Sprintf("%s=%s", key, allTags[key]))
	}
	options := apitypes.ContainerListOptions{
		Filters: filter,
	}
	return &options
}

func (p dockerInstancePlugin) describeInstances(tags map[string]string) ([]instance.Description, error) {

	options := describeGroupRequest(p.namespaceTags, tags)
	ctx := context.Background()
	containers, err := p.client.ContainerList(ctx, *options)
	if err != nil {
		return nil, err
	}

	descriptions := []instance.Description{}
	for _, container := range containers {
		tags := map[string]string{}
		if container.Labels != nil {
			for key, value := range container.Labels {
				tags[key] = value
			}
		}
		if len(container.Names) > 1 {
			log.Debugf("container has %d name(s)", len(container.Names))
		}
		lid := (instance.LogicalID)(strings.TrimPrefix(container.Names[0], "/"))
		descriptions = append(descriptions, instance.Description{
			ID:        instance.ID(container.ID[0:12]),
			LogicalID: &lid,
			Tags:      tags,
		})
	}

	return descriptions, nil
}

// DescribeInstances implements instance.Provisioner.DescribeInstances.
func (p dockerInstancePlugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	return p.describeInstances(tags)
}

// Keys implements the metadata.Plugin SPI's Keys method
func (p dockerInstancePlugin) Keys(path types.Path) ([]string, error) {
	if p.metadataPlugin != nil {
		return p.metadataPlugin.Keys(path)
	}
	return nil, nil
}

// Get implements the metadata.Plugin SPI's Get method
func (p dockerInstancePlugin) Get(path types.Path) (*types.Any, error) {
	if p.metadataPlugin != nil {
		return p.metadataPlugin.Get(path)
	}
	return nil, nil
}
