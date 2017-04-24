package hyperkit

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	hyperkit_go "github.com/moby/hyperkit/go"
	"github.com/rneugeba/iso9660wrap"
)

var log = logutil.New("module", "instance/hyperkit")

// NewPlugin creates an instance plugin for hyperkit.
func NewPlugin(vmDir, hyperkit, vpnkitSock string) instance.Plugin {
	return &hyperkitPlugin{VMDir: vmDir,
		HyperKit:   hyperkit,
		VPNKitSock: vpnkitSock,
		DiskDir:    path.Join(vmDir, "disks"),
	}
}

type hyperkitPlugin struct {
	// VMDir is the path to a directory where per VM state is kept
	VMDir string

	// Hyperkit is the path to the hyperkit executable
	HyperKit string

	// VPNKitSock is the path to the VPNKit Unix domain socket.
	VPNKitSock string

	// DiskDir is the path to persistent (across reboots) disk images
	DiskDir string
}

// Validate performs local validation on a provision request.
func (p hyperkitPlugin) Validate(req *types.Any) error {
	// The guest is just the same data structure used by hyperkit for full fidelity config
	guest := hyperkit_go.HyperKit{}

	if err := req.Decode(&guest); err != nil {
		return fmt.Errorf("error decoding guest configuration: %s", req.String())
	}

	for key, check := range map[string]int{
		"CPUs":     guest.CPUs,
		"Memory":   guest.Memory,
		"DiskSize": guest.DiskSize,
	} {
		if check == 0 {
			return fmt.Errorf("no %s specified", key)
		}
	}

	for key, check := range map[string]string{
		"Kernel":    guest.Kernel,
		"Initrd":    guest.Initrd,
		"DiskImage": guest.DiskImage,
	} {
		if check == "" {
			return fmt.Errorf("no %s specified", key)
		}
	}

	return nil
}

// Provision creates a new instance.
func (p hyperkitPlugin) Provision(spec instance.Spec) (*instance.ID, error) {

	if spec.Properties == nil {
		return nil, fmt.Errorf("missing properties in spec")
	}

	// The guest is just the same data structure used by hyperkit for full fidelity config
	guest := &hyperkit_go.HyperKit{
		HyperKit:   p.HyperKit,
		VPNKitSock: p.VPNKitSock,
		CmdLine:    "console=ttyS0",
	}

	if err := spec.Properties.Decode(guest); err != nil {
		return nil, fmt.Errorf("error decoding guest configuration: %s", spec.Properties.String())
	}

	// directory for instance state
	instanceDir, err := ioutil.TempDir(p.VMDir, "infrakit-")
	if err != nil {
		return nil, err
	}
	guest.StateDir = instanceDir

	// instance id
	id := instance.ID(path.Base(instanceDir))
	log.Info("new instance", "id", id)

	logicalID := string(id)

	if spec.LogicalID != nil {

		// Assume IP address is the format of the LogicalID
		logicalID = string(*spec.LogicalID)

		// The LogicalID may be a IP address. If so, translate
		// it into a magic UUID which cause VPNKit to assign a
		// fixed IP address
		if ip := net.ParseIP(logicalID); len(ip) > 0 {
			vpnkitkey := make([]byte, 16)
			vpnkitkey[12] = ip.To4()[0]
			vpnkitkey[13] = ip.To4()[1]
			vpnkitkey[14] = ip.To4()[2]
			vpnkitkey[15] = ip.To4()[3]

			guest.VPNKitKey = fmt.Sprintf("%x-%x-%x-%x-%x",
				vpnkitkey[0:4],
				vpnkitkey[4:6],
				vpnkitkey[6:8],
				vpnkitkey[8:10],
				vpnkitkey[10:])
		}
		// If a LogicalID is supplied and the Disk size is
		// non-zero, we place the disk in a special directory
		// so it persists across reboots.
		if guest.DiskSize > 0 {
			guest.DiskImage = path.Join(p.DiskDir, logicalID+".img")
		}
	}

	// if there's init then build an iso image of that
	if spec.Init != "" {

		isoImage := path.Join(instanceDir, "data.iso")
		outfh, err := os.OpenFile(isoImage, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Crit("Cannot create user data ISOs", "err", err)
		}
		err = iso9660wrap.WriteBuffer(outfh, []byte(spec.Init), "config")
		if err != nil {
			log.Crit("Cannot write user data ISO", "err", err)
		}
		outfh.Close()

		guest.ISOImage = isoImage
	}

	guest.Console = hyperkit_go.ConsoleFile

	log.Info("Starting guest", "id", id, "guest", guest,
		"kernel", guest.Kernel, "initrd", guest.Initrd,
		"cpus", guest.CPUs, "memory", guest.Memory, "disksize", guest.DiskSize,
		"image", guest.DiskImage, "isoimage", guest.ISOImage,
		"cmdline", guest.CmdLine)

	err = guest.Start(guest.CmdLine)
	if err != nil {
		return nil, err
	}
	log.Info("Started", "id", id)

	if err := ioutil.WriteFile(path.Join(instanceDir, "logical.id"), []byte(logicalID), 0644); err != nil {
		return nil, err
	}

	tagData, err := types.AnyValue(spec.Tags)
	if err != nil {
		return nil, err
	}

	log.Debug("tags", "id", id, "tags", tagData)
	if err := ioutil.WriteFile(path.Join(instanceDir, "tags"), tagData.Bytes(), 0644); err != nil {
		return nil, err
	}

	return &id, nil
}

// Label labels the instance
func (p hyperkitPlugin) Label(instance instance.ID, labels map[string]string) error {
	instanceDir := path.Join(p.VMDir, string(instance))
	tagFile := path.Join(instanceDir, "tags")
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
	return ioutil.WriteFile(tagFile, encoded.Bytes(), 0644)
}

// Destroy terminates an existing instance.
func (p hyperkitPlugin) Destroy(id instance.ID) error {
	log.Info("Destroying VM", "id", id)

	instanceDir := path.Join(p.VMDir, string(id))
	_, err := os.Stat(instanceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("Instance does not exist")
		}
	}

	h, err := hyperkit_go.FromState(instanceDir)
	if err != nil {
		return err
	}
	err = h.Stop()
	if err != nil {
		return err
	}
	err = h.Remove(false)
	if err != nil {
		return err
	}
	return nil
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (p hyperkitPlugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	files, err := ioutil.ReadDir(p.VMDir)
	if err != nil {
		return nil, err
	}

	descriptions := []instance.Description{}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		instanceDir := path.Join(p.VMDir, file.Name())

		tagData, err := ioutil.ReadFile(path.Join(instanceDir, "tags"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, err
		}

		instanceTags := map[string]string{}
		if err := types.AnyBytes(tagData).Decode(&instanceTags); err != nil {
			return nil, err
		}

		allMatched := true
		for k, v := range tags {
			value, exists := instanceTags[k]
			if !exists || v != value {
				allMatched = false
				break
			}
		}

		if allMatched {
			var logicalID *instance.LogicalID
			id := instance.ID(file.Name())

			h, err := hyperkit_go.FromState(instanceDir)
			if err != nil {
				log.Warn("Could not get instance data", "id", id)
				p.Destroy(id)
				continue
			}

			// Some extra information about the instance
			// TODO - explore the hyperkit api and see what it can expose
			lidData, err := ioutil.ReadFile(path.Join(instanceDir, "logical.id"))
			if err != nil {
				log.Warn("Could not get logical ID", "id", id)
				continue
			}
			lid := instance.LogicalID(lidData)
			logicalID = &lid

			desc := instance.Description{
				ID:        id,
				LogicalID: logicalID,
				Tags:      instanceTags,
			}

			if properties {

				extra := map[string]interface{}{
					"running": h.IsRunning(),
				}

				jsonData, err := ioutil.ReadFile(path.Join(instanceDir, "hyperkit.json"))
				if err != nil {
					log.Warn("Could not load hyperkit.json", "id", id)
					continue
				}

				if err := types.AnyBytes(jsonData).Decode(&extra); err != nil {
					log.Warn("Could not decode hyperkit.json", "id", id)
					continue
				}

				desc.Properties = types.AnyValueMust(extra)
			}

			descriptions = append(descriptions, desc)
		}
	}

	return descriptions, nil
}
