package instance

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math/rand"
	"net"

	log "github.com/Sirupsen/logrus"

	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/pkg/errors"
	"github.com/rneugeba/iso9660wrap"
)

type infrakitMetadataTag struct {
	XMLName xml.Name `xml:"tag"`
	Key     string   `xml:"key"`
	Value   string   `xml:"value"`
}

type infrakitMetadataDiskInfo struct {
	Pool   string `xml:"pool"`
	Volume string `xml:"volume"`
}

type infrakitMetadata struct {
	// Really we would like:
	// <infrakit:metadata xmlns:infrakit="https://github.com/docker/infrakit">...</infrakit:metadata>
	// to be compliant with https://libvirt.org/formatdomain.html#elementsMetadata
	// but the closest we can get with encoding/xml appears to be:
	// <infrakit xmlns="https://github.com/docker/infrakit">...</infrakit>
	// See https://github.com/golang/go/issues/13400,
	// https://github.com/golang/go/issues/9519 and various linked issues.
	XMLName          xml.Name                  `xml:"https://github.com/docker/infrakit infrakit"`
	LogicalID        string                    `xml:"logicalid"`
	Tags             []infrakitMetadataTag     `xml:"tag"`
	MetadataDiskInfo *infrakitMetadataDiskInfo `xml:"metadata-disk,omitempty"`
}

func (d *infrakitMetadata) Unmarshal(doc string) error {
	return xml.Unmarshal([]byte(doc), d)
}

func (d *infrakitMetadata) Marshal() (string, error) {
	doc, err := xml.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", err
	}
	return string(doc), nil
}

type domainMetadata struct {
	Data string `xml:",innerxml"`
}

// domainWithMetadata is libvirtxml/Domain with the addition of the Metadata field
type domainWithMetadata struct {
	libvirtxml.Domain
	Metadata *domainMetadata `xml:"metadata,omitempty"`
}

func (d *domainWithMetadata) Unmarshal(doc string) error {
	return xml.Unmarshal([]byte(doc), d)
}

func (d *domainWithMetadata) Marshal() (string, error) {
	doc, err := xml.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", err
	}
	return string(doc), nil
}

// NewLibvirtPlugin creates an instance plugin for libvirt.
func NewLibvirtPlugin(libvirtURI string) instance.Plugin {
	return &libvirtPlugin{
		URI: libvirtURI,
	}
}

type libvirtPlugin struct {
	// URI is the libvirt resource to connect to
	URI string
}

// Validate performs local validation on a provision request.
func (p libvirtPlugin) Validate(req *types.Any) error {
	return nil
}

// Destructively overwrites meta.Tags
func metaSetTags(meta *infrakitMetadata, tags map[string]string) {
	meta.Tags = []infrakitMetadataTag{}
	for k, v := range tags {
		meta.Tags = append(meta.Tags, infrakitMetadataTag{
			Key:   k,
			Value: v,
		})
	}
}

// Properties is the struct which defines spec.Properties for this instance plugin
type Properties struct {
	// Domains is the libvirt domain description
	Domain libvirtxml.Domain

	// MetadataStoragePool is the libvirt storage pool where metadata ISO images should be placed
	MetadataStoragePool string
}

// Provision creates a new instance.
func (p libvirtPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	properties := Properties{
		MetadataStoragePool: "default",
	}

	if spec.Properties != nil {
		if err := spec.Properties.Decode(&properties); err != nil {
			return nil, errors.Errorf("Invalid instance properties: %s", err)
		}
	}

	// The name needs to be unique on the host. In the unlikely
	// event it is not then conn.DomainCreateXML below will fail,
	// but we rely on infrakit to try again.
	id := instance.ID(fmt.Sprintf("infrakit-%08x", rand.Uint32()))
	l := log.WithField("instance", id)

	conn, err := libvirt.NewConnect(p.URI)
	if err != nil {
		return nil, errors.Wrap(err, "Connecting to libvirt")
	}
	defer conn.Close()

	metadataPool := properties.MetadataStoragePool
	metadataVol := ""

	if spec.Init != "" {
		p, err := conn.LookupStoragePoolByName(metadataPool)
		if err != nil {
			return nil, errors.Wrapf(err, "Looking up MetadataStoragePool: %s", metadataPool)
		}

		buf := &bytes.Buffer{}

		if err := iso9660wrap.WriteBuffer(buf, []byte(spec.Init), "config"); err != nil {
			return nil, errors.Wrap(err, "Writing user data ISO")
		}

		len := uint64(buf.Len())

		metadataVol = string(id) + "-metadata"

		volcfg := libvirtxml.StorageVolume{
			Name: metadataVol,
			Capacity: &libvirtxml.StorageVolumeSize{
				Value: len,
				Unit:  "bytes",
			},
		}

		xmldoc, err := volcfg.Marshal()
		if err != nil {
			return nil, errors.Wrap(err, "Marshalling Instance Volume XML")
		}

		vol, err := p.StorageVolCreateXML(xmldoc, 0)
		if err != nil {
			return nil, errors.Wrapf(err, "Creating metadata volume")
		}
		defer func() {
			if metadataVol != "" {
				_ = vol.Delete(0)
			}
		}()

		stream, err := conn.NewStream(0)
		if err != nil {
			return nil, errors.Wrapf(err, "Creating metadata stream")
		}

		if err := vol.Upload(stream, 0, len, 0); err != nil {
			return nil, errors.Wrapf(err, "Starting metadata volume upload")
		}

		if _, err := stream.Send(buf.Bytes()); err != nil {
			return nil, errors.Wrapf(err, "Writing to metadata stream")
		}
	}

	// XXX Check properties.DomainChannels for a Type==unix and Target.Name == "org.qemu.guest_agent.0"

	domcfg := domainWithMetadata{
		Domain: properties.Domain,
	}

	if properties.Domain.Name != "" {
		l.Warnf("Overriding 'Name' property")
	}
	domcfg.Domain.Name = string(id)

	logicalID := string(id)
	if spec.LogicalID != nil {
		logicalID = string(*spec.LogicalID)
		_, err := net.ParseMAC(logicalID)
		if err == nil && domcfg.Domain.Devices != nil && len(domcfg.Domain.Devices.Interfaces) > 0 {
			if domcfg.Domain.Devices.Interfaces[0].MAC != nil {
				l.Warnf("Overriding MAC address of first interface (%q) with logical ID (%q)",
					domcfg.Domain.Devices.Interfaces[0].MAC.Address, logicalID)
			}
			domcfg.Domain.Devices.Interfaces[0].MAC = &libvirtxml.DomainInterfaceMAC{
				Address: logicalID,
			}
		} else {
			l.Warnf("No Network device to apply logical ID %q to", logicalID)
		}
	}

	meta := infrakitMetadata{
		LogicalID: logicalID,
	}
	metaSetTags(&meta, spec.Tags)

	if metadataPool != "" && metadataVol != "" {
		l.Printf("Adding %s %s as metadata", metadataPool, metadataVol)
		domcfg.Domain.Devices.Disks = append(domcfg.Domain.Devices.Disks, libvirtxml.DomainDisk{
			Type:   "volume",
			Device: "cdrom",
			Source: &libvirtxml.DomainDiskSource{
				Pool:   metadataPool,
				Volume: metadataVol,
			},
			Target: &libvirtxml.DomainDiskTarget{
				Dev: "sdc",
				Bus: "sata",
			},
		})

		meta.MetadataDiskInfo = &infrakitMetadataDiskInfo{
			Pool:   metadataPool,
			Volume: metadataVol,
		}
	}

	m, err := meta.Marshal()
	if err != nil {
		return nil, errors.Wrap(err, "Marshalling infrakitMetadata")
	}
	domcfg.Metadata = &domainMetadata{m}

	xmldoc, err := domcfg.Marshal()
	if err != nil {
		l.Errorf("Marshalling Domain XML: %s", err)
		return nil, errors.Wrap(err, "Marshalling Domain XML")
	}

	l.Debug(xmldoc)

	dom, err := conn.DomainCreateXML(string(xmldoc), 0)
	if err != nil {
		l.Errorf("Creating Domain: %s", err)
		return nil, errors.Wrap(err, "Creating Domain")
	}

	domid, err := dom.GetID()
	if err != nil {
		l.Errorf("Getting Domain ID: %s", err)
		return nil, errors.Wrap(err, "Getting Domain ID")
	}

	l.Infof("New instance started as dom%d", domid)

	metadataVol = "" // Success, do not clean this up.

	return &id, nil
}

func (p libvirtPlugin) lookupInstanceByID(conn *libvirt.Connect, id instance.ID) (*libvirt.Domain, error) {
	// id is the domain's name
	name := string(id)

	doms, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		return nil, errors.Wrap(err, "Listing all domains")
	}

	for _, d := range doms {
		domName, err := d.GetName()
		if err != nil {
			continue
		}
		if domName == name {
			return &d, nil
		}
	}

	return nil, errors.Errorf("Domain %s not found", name)
}

// Label labels the instance
func (p libvirtPlugin) Label(instance instance.ID, labels map[string]string) error {
	//l := log.WithField("instance", instance)

	conn, err := libvirt.NewConnect(p.URI)
	if err != nil {
		return errors.Wrap(err, "Connecting to libvirt")
	}
	defer conn.Close()

	d, err := p.lookupInstanceByID(conn, instance)
	if err != nil {
		return errors.Wrap(err, "Looking up domain")
	}

	meta := infrakitMetadata{}
	m, err := d.GetMetadata(libvirt.DOMAIN_METADATA_ELEMENT,
		"https://github.com/docker/infrakit",
		libvirt.DOMAIN_AFFECT_LIVE)
	if err == nil {
		if err := meta.Unmarshal(m); err != nil {
			return errors.Wrap(err, "Unmarshalling domain metadata XML")
		}
	} else {
		meta.LogicalID = string(instance)
	}

	metaSetTags(&meta, labels)

	xmlbytes, err := xml.MarshalIndent(meta, "", "  ")
	if err != nil {
		return errors.Wrap(err, "Marshalling infrakitMetadata")
	}
	m = string(xmlbytes)

	err = d.SetMetadata(libvirt.DOMAIN_METADATA_ELEMENT,
		m,
		"infrakit",
		"https//github.com/docker/infrakit",
		libvirt.DOMAIN_AFFECT_LIVE)
	if err != nil {
		return errors.Wrap(err, "Setting domain metadata")
	}

	return nil
}

func destroyMetadataDisk(conn *libvirt.Connect, d *libvirt.Domain) error {
	xmldoc, err := d.GetXMLDesc(0)
	if err != nil {
		return errors.Wrap(err, "Getting domain XML")
	}
	var domcfg domainWithMetadata
	if err := domcfg.Unmarshal(xmldoc); err != nil {
		return errors.Wrap(err, "Unmarshalling domain XML")
	}

	if domcfg.Metadata == nil {
		return errors.New("Domain is lacking metadata")
	}

	meta := infrakitMetadata{}
	if err := meta.Unmarshal(domcfg.Metadata.Data); err != nil {
		return errors.Wrap(err, "Unmarshalling metadata")
	}

	if meta.MetadataDiskInfo == nil {
		return nil // No metadata disk for this VM
	}

	p, err := conn.LookupStoragePoolByName(meta.MetadataDiskInfo.Pool)
	if err != nil {
		return errors.Wrap(err, "Finding metadata disk's pool")
	}

	v, err := p.LookupStorageVolByName(meta.MetadataDiskInfo.Volume)
	if err != nil {
		return errors.Wrap(err, "Finding metadata disk's volume")
	}
	if err := v.Delete(0); err != nil {
		return errors.Wrap(err, "Deleteing metadata disk's volume")
	}

	return nil
}

// Destroy terminates an existing instance.
func (p libvirtPlugin) Destroy(id instance.ID, ctx instance.Context) error {
	l := log.WithField("instance", id)
	l.Info("Destroying VM")

	conn, err := libvirt.NewConnect(p.URI)
	if err != nil {
		return errors.Wrap(err, "Connecting to libvirt")
	}
	defer conn.Close()

	d, err := p.lookupInstanceByID(conn, id)
	if err != nil {
		return errors.Wrap(err, "Looking up domain")
	}

	if err := destroyMetadataDisk(conn, d); err != nil {
		l.Errorf("Failed to destroy metadata disk: %s", err)
		// Continue so we at least try and destroy the domain
	}

	if err := d.Destroy(); err != nil {
		return errors.Wrap(err, "Destroying domain")
	}

	return nil
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (p libvirtPlugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	conn, err := libvirt.NewConnect(p.URI)
	if err != nil {
		return nil, errors.Wrap(err, "Connecting to libvirt")
	}
	defer conn.Close()

	doms, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		return nil, errors.Wrap(err, "Listing all domains")
	}

	var descriptions []instance.Description
	for _, d := range doms {

		info, err := d.GetInfo()
		if err != nil {
			return nil, errors.Wrap(err, "Getting domain info")
		}
		if info.State != libvirt.DOMAIN_RUNNING {
			continue
		}
		xmldoc, err := d.GetXMLDesc(0)
		if err != nil {
			return nil, errors.Wrap(err, "Getting domain XML")
		}
		var domcfg domainWithMetadata
		if err := domcfg.Unmarshal(xmldoc); err != nil {
			return nil, errors.Wrap(err, "Unmarshalling domain XML")
		}

		meta := infrakitMetadata{}
		if domcfg.Metadata != nil {
			if err := meta.Unmarshal(domcfg.Metadata.Data); err != nil {
				// Assume it is not one of ours.
				continue
			}
		} else {
			// Assume it is not one of ours.
			continue
		}

		instanceTags := make(map[string]string)
		for _, tag := range meta.Tags {
			instanceTags[tag.Key] = tag.Value
		}

		allMatched := true
		for k, v := range tags {
			value, exists := instanceTags[k]
			if !exists || v != value {
				allMatched = false
				break
			}
		}
		lid := instance.LogicalID(meta.LogicalID)
		if allMatched {
			description := instance.Description{
				ID:        instance.ID(domcfg.Name),
				LogicalID: &lid,
				Tags:      instanceTags,
			}

			if properties {
				extra := map[string]interface{}{}

				srcs := []libvirt.DomainInterfaceAddressesSource{
					libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_AGENT,
					libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE,
				}

				for _, src := range srcs {
					if allifaces, err := d.ListAllInterfaceAddresses(src); err == nil {
						ifaces := []libvirt.DomainInterface{}

						for _, iface := range allifaces {
							// Filter out uninteresting interfaces with no addresses
							if iface.Hwaddr == "" || len(iface.Addrs) == 0 {
								continue
							}
							ifaces = append(ifaces, iface)
						}

						if len(ifaces) > 0 {
							extra["interfaces"] = ifaces
							break
						}
					}
				}
				description.Properties = types.AnyValueMust(extra)
			}

			descriptions = append(descriptions, description)
		}

	}

	return descriptions, nil
}
