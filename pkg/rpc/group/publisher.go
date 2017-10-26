package group

import (
	"fmt"

	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/types"
)

// List returns a list of *child nodes* given a path for a topic.
// A topic of "." is the top level
func (p *Group) List(topic types.Path) (child []string, err error) {
	fmt.Println(">>>>>> list")
	return []string{"error", "provision", "destroy", "health", "drain", "prepare"}, nil
}

// PublishOn sets the channel to publish
func (p *Group) PublishOn(chan<- *event.Event) {
	fmt.Println(">>>>> publishOn")
}
