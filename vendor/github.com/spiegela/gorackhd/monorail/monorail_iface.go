package monorail

import (
	"github.com/go-openapi/runtime"
	"github.com/spiegela/gorackhd/client/nodes"
	"github.com/spiegela/gorackhd/client/skus"
	"github.com/spiegela/gorackhd/client/tags"
)

// Iface provides an interface to enable mocking
type Iface interface {
	Login(string, string) (runtime.ClientAuthInfoWriter, error)
	Nodes() NodeIface
	Skus() SkuIface
	Tags() TagIface
}

// NodeIface provides an interface to enable mocking
type NodeIface interface {
	NodesAddRelations(params *nodes.NodesAddRelationsParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesAddRelationsOK, error)
	NodesDelByID(params *nodes.NodesDelByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesDelByIDNoContent, error)
	NodesDelRelations(params *nodes.NodesDelRelationsParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesDelRelationsNoContent, error)
	NodesDelTagByID(params *nodes.NodesDelTagByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesDelTagByIDNoContent, error)
	NodesGetAll(params *nodes.NodesGetAllParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesGetAllOK, error)
	NodesGetByID(params *nodes.NodesGetByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesGetByIDOK, error)
	NodesGetCatalogByID(params *nodes.NodesGetCatalogByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesGetCatalogByIDOK, error)
	NodesGetCatalogSourceByID(params *nodes.NodesGetCatalogSourceByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesGetCatalogSourceByIDOK, error)
	NodesGetObmsByNodeID(params *nodes.NodesGetObmsByNodeIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesGetObmsByNodeIDOK, error)
	NodesGetPollersByID(params *nodes.NodesGetPollersByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesGetPollersByIDOK, error)
	NodesGetRelations(params *nodes.NodesGetRelationsParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesGetRelationsOK, error)
	NodesGetSSHByID(params *nodes.NodesGetSSHByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesGetSSHByIDOK, error)
	NodesGetTagsByID(params *nodes.NodesGetTagsByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesGetTagsByIDOK, error)
	NodesGetWorkflowByID(params *nodes.NodesGetWorkflowByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesGetWorkflowByIDOK, error)
	NodesMasterDelTagByID(params *nodes.NodesMasterDelTagByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesMasterDelTagByIDNoContent, error)
	NodesPatchByID(params *nodes.NodesPatchByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesPatchByIDOK, error)
	NodesPatchTagByID(params *nodes.NodesPatchTagByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesPatchTagByIDOK, error)
	NodesPost(params *nodes.NodesPostParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesPostCreated, error)
	NodesPostSSHByID(params *nodes.NodesPostSSHByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesPostSSHByIDCreated, error)
	NodesPostWorkflowByID(params *nodes.NodesPostWorkflowByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesPostWorkflowByIDCreated, error)
	NodesPutObmsByNodeID(params *nodes.NodesPutObmsByNodeIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesPutObmsByNodeIDCreated, error)
	NodesWorkflowActionByID(params *nodes.NodesWorkflowActionByIDParams, authInfo runtime.ClientAuthInfoWriter) (*nodes.NodesWorkflowActionByIDAccepted, error)
}

// SkuIface provides an interface to enable mocking
type SkuIface interface {
	SkusGet(params *skus.SkusGetParams, authInfo runtime.ClientAuthInfoWriter) (*skus.SkusGetOK, error)
	SkuPackPost(params *skus.SkuPackPostParams, authInfo runtime.ClientAuthInfoWriter) (*skus.SkuPackPostCreated, error)
	SkusIDDelete(params *skus.SkusIDDeleteParams, authInfo runtime.ClientAuthInfoWriter) (*skus.SkusIDDeleteNoContent, error)
	SkusIDDeletePack(params *skus.SkusIDDeletePackParams, authInfo runtime.ClientAuthInfoWriter) (*skus.SkusIDDeletePackNoContent, error)
	SkusIDGet(params *skus.SkusIDGetParams, authInfo runtime.ClientAuthInfoWriter) (*skus.SkusIDGetOK, error)
	SkusIDGetNodes(params *skus.SkusIDGetNodesParams, authInfo runtime.ClientAuthInfoWriter) (*skus.SkusIDGetNodesOK, error)
	SkusIDPutPack(params *skus.SkusIDPutPackParams, authInfo runtime.ClientAuthInfoWriter) (*skus.SkusIDPutPackCreated, error)
	SkusPatch(params *skus.SkusPatchParams, authInfo runtime.ClientAuthInfoWriter) (*skus.SkusPatchOK, error)
	SkusPost(params *skus.SkusPostParams, authInfo runtime.ClientAuthInfoWriter) (*skus.SkusPostCreated, error)
	SkusPut(params *skus.SkusPutParams, authInfo runtime.ClientAuthInfoWriter) (*skus.SkusPutCreated, error)
}

// TagIface provides an interface to enable mocking
type TagIface interface {
	CreateTag(params *tags.CreateTagParams, authInfo runtime.ClientAuthInfoWriter) (*tags.CreateTagCreated, error)
	DeleteTag(params *tags.DeleteTagParams, authInfo runtime.ClientAuthInfoWriter) (*tags.DeleteTagNoContent, error)
	GetAllTags(params *tags.GetAllTagsParams, authInfo runtime.ClientAuthInfoWriter) (*tags.GetAllTagsOK, error)
	GetNodesByTag(params *tags.GetNodesByTagParams, authInfo runtime.ClientAuthInfoWriter) (*tags.GetNodesByTagOK, error)
	GetTag(params *tags.GetTagParams, authInfo runtime.ClientAuthInfoWriter) (*tags.GetTagOK, error)
	PostWorkflowByID(params *tags.PostWorkflowByIDParams, authInfo runtime.ClientAuthInfoWriter) (*tags.PostWorkflowByIDAccepted, error)
}
