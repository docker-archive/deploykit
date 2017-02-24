package instance

import(
  "github.com/codedellemc/gorackhd/client"
  "github.com/docker/infrakit/pkg/spi/instance"
)

type rackhdInstancePlugin struct {
  client client.Monorail
}

func NewInstancePlugin(client client.Monorail) instance.Plugin {
  return &rackhdInstancePlugin{client: client}
}
