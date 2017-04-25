package libvirt

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
	descrs, err := plugin.DescribeInstances(map[string]string{}, false)
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

func TestBasicLifecycle(t *testing.T) {
	conn, err := libvirt.NewConnect(libvirtDrvTest)
	require.NoError(t, err)
	defer conn.Close()

	plugin := NewLibvirtPlugin(libvirtDrvTest)
	require.NotNil(t, plugin)

	props := types.Any(`{
    "DomainType": "test",
    "Kernel": "/foo/bar/Image",
    "CPUs": 1,
    "Memory" : 1024
}`)

	logicalID := instance.LogicalID("spock")

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

	err = plugin.Destroy(*id)
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

	labels := map[string]string{}
	err = plugin.Label(instance.ID(domname), labels)
	require.NoError(t, err)

	require.True(t, instanceExists(t, plugin, instance.ID(domname)), "domain was not found in plugin instances after label")

	inst := findInstance(t, plugin, instance.ID(domname))
	require.Equal(t, labels, inst.Tags)
}
