package group

import (
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/types"
)

// List returns a list of *child nodes* given a path for a topic.
// A topic of "." is the top level
func (p *Group) List(topic types.Path) (child []string, err error) {
	m := map[string]interface{}{}

	subs := p.keyed.Objects()
	if len(subs) > 0 {
		for _, o := range subs {
			types.Put([]string{o.Name, "commit"}, "", m)
			types.Put([]string{o.Name, "describe"}, "", m)
		}
	} else {
		types.Put([]string{"commit"}, "", m)
		types.Put([]string{"describe"}, "", m)
	}

	return types.List(topic, m), nil
}

// PublishOn sets the channel to publish
func (p *Group) PublishOn(chan<- *event.Event) {
}
