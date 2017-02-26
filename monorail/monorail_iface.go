package monorail

import (
	"github.com/codedellemc/gorackhd/client/nodes"
	"github.com/go-openapi/runtime"
)

// Iface provides an interface to enable mocking
type Iface interface {
	Login(string, string) (runtime.ClientAuthInfoWriter, error)
	Nodes() NodeIface
}

// NodeIface provides an interface to enable mocking
type NodeIface interface {
	GetNodes(*nodes.GetNodesParams, runtime.ClientAuthInfoWriter) (*nodes.GetNodesOK, error)
}
