package event

import (
	"net/http"
	"sort"

	broker "github.com/docker/infrakit/pkg/broker/server"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/types"
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

func self(p types.Path) bool {
	if p.Len() == 0 {
		return true
	}

	first := p.Index(0)
	if first == nil {
		return true
	}

	switch *first {
	case "", ".":
		return true
	}

	return false
}

// Validate returns an error if the topic path is invalid
func (p *Event) Validate(topic types.Path) error {
	// case where the topic can have sub topics
	children, err := p.list(topic)
	if err != nil {
		return err
	}

	// It can be zero length
	if children != nil {
		return nil
	}

	// case where the topic is exact
	children, err = p.list(topic.Dir())
	if err != nil {
		return err
	}
	if len(children) > 0 && children[0] == topic.Base() {
		return nil
	}

	return broker.ErrInvalidTopic(topic.String())
}

func (p *Event) list(topic types.Path) ([]string, error) {
	nodes := []string{}
	// the . case - list the typed plugins and the default's first level.
	if self(topic) {
		if p.plugin != nil {
			n, err := p.plugin.List(topic)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, n...)
		}
		for k := range p.typedPlugins {
			nodes = append(nodes, k)
		}
		sort.Strings(nodes)
		return nodes, nil
	}

	c, has := p.typedPlugins[topic[0]]
	if !has {

		if p.plugin == nil {
			return nil, nil
		}

		nodes, err := p.plugin.List(topic)
		if err != nil {
			return nil, err
		}
		sort.Strings(nodes)
		return nodes, nil
	}

	nodes, err := c.List(topic[1:])
	if err != nil {
		return nil, err
	}

	sort.Strings(nodes)
	return nodes, nil
}

// List return a set of sub topics given the top level one
func (p *Event) List(_ *http.Request, req *ListRequest, resp *ListResponse) error {
	req.Topic = req.Topic.Clean()
	nodes, err := p.list(req.Topic)
	if err != nil {
		return err
	}
	resp.Nodes = nodes
	return nil
}

func asPublisher(p event.Plugin) event.Publisher {
	if pub, is := p.(event.Publisher); is {
		return pub
	}
	return nil
}

// PublishOn sets the publish function of the plugin
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
						e.Topic = types.PathFromString(namespace).Join(e.Topic)
						c <- e
					} else {
						return
					}
				}
			}()
		}
	}
}
