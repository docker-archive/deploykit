package libvirt

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"math/rand"
	"path/filepath"

	log "github.com/Sirupsen/logrus"

	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/pkg/errors"
)

type infrakitMetadataTag struct {
	XMLName xml.Name `xml:"tag"`
	Key     string   `xml:"key"`
	Value   string   `xml:"value"`
}

type infrakitMetadata struct {
	// Really we would like:
	// <infrakit:metadata xmlns:infrakit="https://github.com/docker/infrakit">...</infrakit:metadata>
	// to be compliant with https://libvirt.org/formatdomain.html#elementsMetadata
	// but the closest we can get with encoding/xml appears to be:
	// <infrakit xmlns="https://github.com/docker/infrakit">...</infrakit>
	// See https://github.com/golang/go/issues/13400,
	// https://github.com/golang/go/issues/9519 and various linked issues.
	XMLName   xml.Name              `xml:"https://github.com/docker/infrakit infrakit"`
	LogicalID string                `xml:"logicalid"`
	Tags      []infrakitMetadataTag `xml:"tag"`
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

// Provision creates a new instance.
func (p libvirtPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	var properties map[string]interface{}

	if spec.Properties != nil {
		if err := spec.Properties.Decode(&properties); err != nil {
			return nil, errors.Errorf("Invalid instance properties: %s", err)
		}
	}

	if properties["Kernel"] == nil {
		return nil, errors.New("Property 'Kernel' must be set")
	}
	if properties["CPUs"] == nil {
		properties["CPUs"] = 1
	}
	if properties["Memory"] == nil {
		properties["Memory"] = 512
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

	domtype, ok := properties["DomainType"].(string)
	if !ok {
		domtype = "kvm"
	}
	kernel, err := filepath.Abs(properties["Kernel"].(string))
	if err != nil {
		return nil, errors.Wrap(err, "Constructing path to kernel")
	}
	initrd := ""
	if initrdprop, ok := properties["Ramdisk"].(string); ok {
		initrd, err = filepath.Abs(initrdprop)
		if err != nil {
			return nil, errors.Wrap(err, "Constructing path to initrd")
		}
	}
	cmdline := ""
	if properties["Cmdline"] != nil {
		cmdline = properties["Cmdline"].(string)
	} else if properties["CmdlineFile"] != nil {
		f := properties["CmdlineFile"].(string)
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, errors.Wrapf(err, "Reading CmdlineFile: %s", f)
		}
		cmdline = string(b)
	}

	domcfg := domainWithMetadata{
		Domain: libvirtxml.Domain{
			Type:   domtype,
			VCPU:   &libvirtxml.DomainVCPU{Value: int(properties["CPUs"].(float64))},
			Memory: &libvirtxml.DomainMemory{Value: int(properties["Memory"].(float64)), Unit: "MiB"},
			Name:   string(id),
			OS: &libvirtxml.DomainOS{
				Type: &libvirtxml.DomainOSType{
					Type: "hvm",
				},
				Kernel:     kernel,
				Initrd:     initrd,
				KernelArgs: string(cmdline),
				BIOS: &libvirtxml.DomainBIOS{
					UseSerial: "yes",
				},
			},
			Devices: &libvirtxml.DomainDeviceList{
				Serials: []libvirtxml.DomainChardev{
					{
						Type: "pty",
					},
				},
				Consoles: []libvirtxml.DomainChardev{
					{
						Type: "pty",
						Target: &libvirtxml.DomainChardevTarget{
							Type: "serial",
							Name: "0",
						},
					},
				},
			},
			OnCrash:    "destroy",
			OnPoweroff: "destroy",
		},
	}

	logicalID := string(id)
	if spec.LogicalID != nil {
		logicalID = string(*spec.LogicalID)
	}

	meta := infrakitMetadata{
		LogicalID: logicalID,
	}
	metaSetTags(&meta, spec.Tags)
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

// Destroy terminates an existing instance.
func (p libvirtPlugin) Destroy(id instance.ID) error {
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
			descriptions = append(descriptions, instance.Description{
				ID:        instance.ID(domcfg.Name),
				LogicalID: &lid,
				Tags:      instanceTags,
			})
		}

	}

	return descriptions, nil
}
