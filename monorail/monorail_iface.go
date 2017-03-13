package monorail

import (
	"github.com/codedellemc/gorackhd/client/nodes"
	"github.com/codedellemc/gorackhd/client/skus"
	"github.com/go-openapi/runtime"
)

// Iface provides an interface to enable mocking
type Iface interface {
	Login(string, string) (runtime.ClientAuthInfoWriter, error)
	Nodes() NodeIface
	Skus() SkuIface
}

// NodeIface provides an interface to enable mocking
type NodeIface interface {
	GetNodes(*nodes.GetNodesParams, runtime.ClientAuthInfoWriter) (*nodes.GetNodesOK, error)
	PostNodesIdentifierWorkflows(*nodes.PostNodesIdentifierWorkflowsParams, runtime.ClientAuthInfoWriter) (*nodes.PostNodesIdentifierWorkflowsCreated, error)
	PatchNodesIdentifierTags(*nodes.PatchNodesIdentifierTagsParams, runtime.ClientAuthInfoWriter) (*nodes.PatchNodesIdentifierTagsOK, error)
	DeleteNodesIdentifier(*nodes.DeleteNodesIdentifierParams, runtime.ClientAuthInfoWriter) (*nodes.DeleteNodesIdentifierOK, error)
	GetNodesIdentifierObm(*nodes.GetNodesIdentifierObmParams, runtime.ClientAuthInfoWriter) (*nodes.GetNodesIdentifierObmOK, error)
}

// SkuIface provides an interface to enable mocking
type SkuIface interface {
	GetSkus(*skus.GetSkusParams, runtime.ClientAuthInfoWriter) (*skus.GetSkusOK, error)
	GetSkusIdentifierNodes(*skus.GetSkusIdentifierNodesParams, runtime.ClientAuthInfoWriter) (*skus.GetSkusIdentifierNodesOK, error)
}
