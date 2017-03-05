package event

import (
	"net/http"

	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/event"
)

// PluginServer returns a Event that conforms to the net/rpc rpc call convention.
func PluginServer(p event.Plugin) *Event {
	return &Event{plugin: p}
}

// PluginServerWithTypes which supports multiple types of event plugins. The de-multiplexing
// is done by the server's RPC method implementations.
func PluginServerWithTypes(typed map[string]event.Plugin) *Event {
	return &Event{typedPlugins: typed}
}

// Event the exported type needed to conform to json-rpc call convention
type Event struct {
	plugin       event.Plugin
	typedPlugins map[string]event.Plugin // by type, as qualified in the name of the plugin
}

// WithBase sets the base plugin to the given plugin object
func (p *Event) WithBase(m event.Plugin) *Event {
	p.plugin = m
	return p
}

// WithTypes sets the typed plugins to the given map of plugins (by type name)
func (p *Event) WithTypes(typed map[string]event.Plugin) *Event {
	p.typedPlugins = typed
	return p
}

// VendorInfo returns a event object about the plugin, if the plugin implements it.  See spi.Vendor
func (p *Event) VendorInfo() *spi.VendorInfo {
	// TODO(chungers) - support typed plugins
	if p.plugin == nil {
		return nil
	}

	if m, is := p.plugin.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *Event) ImplementedInterface() spi.InterfaceSpec {
	return event.InterfaceSpec
}

func (p *Event) getPlugin(eventType string) event.Plugin {
	if eventType == "" {
		return p.plugin
	}
	if p, has := p.typedPlugins[eventType]; has {
		return p
	}
	return nil
}

// Topics return a set of topics
func (p *Event) Topics(_ *http.Request, req *TopicsRequest, resp *TopicsResponse) error {
	// We need to aggregate the events exposed by all the plugin objects
	topics := []event.Topic{}

	if p.plugin != nil {
		n, err := p.plugin.Topics()
		if err == nil {
			topics = append(topics, n...)
		}
	}

	for typeName, typed := range p.typedPlugins {
		parent := event.Topic(typeName)
		if n, err := typed.Topics(); err == nil {
			for _, t := range n {
				// need to scope the topic by the name of the plugin
				topics = append(topics, t.Under(parent))
			}
		}
	}
	event.Sort(topics)
	resp.Topics = topics
	return nil
}

func asPublisher(p event.Plugin) event.Publisher {
	if pub, is := p.(event.Publisher); is {
		return pub
	}
	return nil
}

// PublishChan sets the publish function of the plugin
func (p *Event) PublishOn(c chan<- *event.Event) {

	if pub := asPublisher(p.plugin); pub != nil {
		pub.PublishOn(c)
	}

	for name, typed := range p.typedPlugins {
		if pub := asPublisher(typed); pub != nil {

			cc := make(chan *event.Event)
			pub.PublishOn(cc)
			namespace := name

			go func() {
				for {
					if e, ok := <-cc; ok {
						e.Topic = e.Topic.Under(event.NewTopic(namespace))
						c <- e
					} else {
						return
					}
				}
			}()
		}
	}
}
