package instance

import (
	"testing"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"
	"github.com/stretchr/testify/require"
)

const libvirtDrvTest = "test:///default"

// checks if a domain with the given name exists from libvirt's PoV
func domainExists(t *testing.T, conn *libvirt.Connect, id string) bool {
	doms, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	require.NoError(t, err)

	for _, d := range doms {
		info, err := d.GetInfo()
		require.NoError(t, err)

		if info.State != libvirt.DOMAIN_RUNNING {
			continue
		}

		name, err := d.GetName()
		require.NoError(t, err)
		if name == id {
			return true
		}
	}
	return false
}

func findInstance(t *testing.T, plugin instance.Plugin, id instance.ID) *instance.Description {
	descrs, err := plugin.DescribeInstances(map[string]string{}, true)
	require.NoError(t, err)

	for _, d := range descrs {
		if d.ID == id {
			return &d
		}
	}
	return nil
}

// checks if an instance exists from the plugin's PoV
func instanceExists(t *testing.T, plugin instance.Plugin, id instance.ID) bool {
	return findInstance(t, plugin, id) != nil
}

func TestInvalidProperties(t *testing.T) {
	conn, err := libvirt.NewConnect(libvirtDrvTest)
	require.NoError(t, err)
	defer conn.Close()

	plugin := NewLibvirtPlugin(libvirtDrvTest)
	require.NotNil(t, plugin)

	props := types.Any(`this is not valid JSON`)

	logicalID := instance.LogicalID("")

	_, err = plugin.Provision(instance.Spec{
		Properties:  &props,
		Tags:        map[string]string{},
		Init:        "",
		LogicalID:   &logicalID,
		Attachments: []instance.Attachment{},
	})
	require.Error(t, err)
}

func TestBasicLifecycle(t *testing.T) {
	conn, err := libvirt.NewConnect(libvirtDrvTest)
	require.NoError(t, err)
	defer conn.Close()

	plugin := NewLibvirtPlugin(libvirtDrvTest)
	require.NotNil(t, plugin)

	props := types.Any(`{
    "Domain": {
        "Type": "test",
        "VCPU": {
            "Value": 1
        },
        "Memory" : {
            "Unit": "MiB",
            "Value": 1024
        },
        "OS": {
            "Type": {
                "Type": "hvm"
            },
            "Kernel": "/foo/bar/Image"
         },
         "Devices": {
            "Interfaces": [
                {
                    "Type": "bridge",
                    "Source": {
                        "Bridge": "virbr0"
                    }
                }
             ]
         }
    }
}`)

	logicalID := instance.LogicalID("da:37:60:0c:e2:fe")

	id, err := plugin.Provision(instance.Spec{
		Properties:  &props,
		Tags:        map[string]string{},
		Init:        "",
		LogicalID:   &logicalID,
		Attachments: []instance.Attachment{},
	})
	require.NoError(t, err)
	require.NotNil(t, id)

	require.True(t, domainExists(t, conn, string(*id)), "domain was not found in libvirt domains")
	require.True(t, instanceExists(t, plugin, *id), "domain was not found in plugin instances")

	inst := findInstance(t, plugin, *id)
	require.NotNil(t, inst)

	require.Equal(t, logicalID, *inst.LogicalID)

	err = plugin.Destroy(*id, instance.Termination)
	require.NoError(t, err)

	require.False(t, domainExists(t, conn, string(*id)), "domain was found in libvirt domains after destroy")
	require.False(t, instanceExists(t, plugin, *id), "domain was found in plugin instances after destroy")
}

// TestNoDevices checks that we do not crash if props.Domain.Devices is not populated.
func TestNoDevices(t *testing.T) {
	conn, err := libvirt.NewConnect(libvirtDrvTest)
	require.NoError(t, err)
	defer conn.Close()

	plugin := NewLibvirtPlugin(libvirtDrvTest)
	require.NotNil(t, plugin)

	props := types.Any(`{
    "Domain": {
        "Type": "test",
        "VCPU": {
            "Value": 1
        },
        "Memory" : {
            "Unit": "MiB",
            "Value": 1024
        },
        "OS": {
            "Type": {
                "Type": "hvm"
            },
            "Kernel": "/foo/bar/Image"
         }
    }
}`)

	logicalID := instance.LogicalID("d2:60:fb:f5:bd:44")

	id, err := plugin.Provision(instance.Spec{
		Properties:  &props,
		Tags:        map[string]string{},
		Init:        "",
		LogicalID:   &logicalID,
		Attachments: []instance.Attachment{},
	})
	require.NoError(t, err)
	require.NotNil(t, id)

	require.True(t, domainExists(t, conn, string(*id)), "domain was not found in libvirt domains")
	require.True(t, instanceExists(t, plugin, *id), "domain was not found in plugin instances")

	inst := findInstance(t, plugin, *id)
	require.NotNil(t, inst)

	require.Equal(t, logicalID, *inst.LogicalID)

	err = plugin.Destroy(*id, instance.Termination)
	require.NoError(t, err)

	require.False(t, domainExists(t, conn, string(*id)), "domain was found in libvirt domains after destroy")
	require.False(t, instanceExists(t, plugin, *id), "domain was found in plugin instances after destroy")
}

// TestLogicalIDNotMAC checks that we do not fail if LogicalID happens to not be a MAC address
func TestLogicalIDNotMAC(t *testing.T) {
	conn, err := libvirt.NewConnect(libvirtDrvTest)
	require.NoError(t, err)
	defer conn.Close()

	plugin := NewLibvirtPlugin(libvirtDrvTest)
	require.NotNil(t, plugin)

	props := types.Any(`{
    "Domain": {
        "Type": "test",
        "VCPU": {
            "Value": 1
        },
        "Memory" : {
            "Unit": "MiB",
            "Value": 1024
        },
        "OS": {
            "Type": {
                "Type": "hvm"
            },
            "Kernel": "/foo/bar/Image"
         }
    }
}`)

	logicalID := instance.LogicalID("da:37:60:0c:e2:fe")

	id, err := plugin.Provision(instance.Spec{
		Properties:  &props,
		Tags:        map[string]string{},
		Init:        "",
		LogicalID:   &logicalID,
		Attachments: []instance.Attachment{},
	})
	require.NoError(t, err)
	require.NotNil(t, id)

	require.True(t, domainExists(t, conn, string(*id)), "domain was not found in libvirt domains")
	require.True(t, instanceExists(t, plugin, *id), "domain was not found in plugin instances")

	inst := findInstance(t, plugin, *id)
	require.NotNil(t, inst)

	require.Equal(t, logicalID, *inst.LogicalID)

	err = plugin.Destroy(*id, instance.Termination)
	require.NoError(t, err)

	require.False(t, domainExists(t, conn, string(*id)), "domain was found in libvirt domains after destroy")
	require.False(t, instanceExists(t, plugin, *id), "domain was found in plugin instances after destroy")
}

func TestLabel(t *testing.T) {
	conn, err := libvirt.NewConnect(libvirtDrvTest)
	require.NoError(t, err)
	defer conn.Close()

	domname := "test-instanceXXXX"

	domcfg := libvirtxml.Domain{
		Type:   "test",
		VCPU:   &libvirtxml.DomainVCPU{Value: 1},
		Memory: &libvirtxml.DomainMemory{Value: 1024, Unit: "MiB"},
		Name:   "test-instanceXXXX",
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Type: "hvm",
			},
			Kernel:     "/foo/bar/Kernel",
			Initrd:     "/foo/bar/Ramdisk",
			KernelArgs: "root=/dev/sda2",
		},
	}

	xmldoc, err := domcfg.Marshal()
	require.NoError(t, err)

	_, err = conn.DomainCreateXML(string(xmldoc), 0)
	require.NoError(t, err)

	require.True(t, domainExists(t, conn, domname), "domain was not found in libvirt domains")

	plugin := NewLibvirtPlugin(libvirtDrvTest)
	require.NotNil(t, plugin)

	require.False(t, instanceExists(t, plugin, instance.ID(domname)), "domain was found in plugin instances before label")

	labels := map[string]string{
		"foo":  "bar",
		"foo2": "bar",
	}
	err = plugin.Label(instance.ID(domname), labels)
	require.NoError(t, err)

	require.True(t, instanceExists(t, plugin, instance.ID(domname)), "domain was not found in plugin instances after label")

	inst := findInstance(t, plugin, instance.ID(domname))
	require.Equal(t, labels, inst.Tags)
}
