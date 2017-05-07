package metadata

import (
	"fmt"
	"net/http"

	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// UpdatablePluginServer returns a Metadata that conforms to the net/rpc rpc call convention.
func UpdatablePluginServer(p metadata.Updatable) *Updatable {
	return &Updatable{Metadata: &Metadata{plugin: p}, updatable: p}
}

// Updatable is a rpc object with methods that are exported as JSON RPC service methods
type Updatable struct {
	*Metadata
	updatable metadata.Updatable
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (u *Updatable) ImplementedInterface() spi.InterfaceSpec {
	return metadata.UpdatableInterfaceSpec
}

// List returns a list of child nodes given a path.
func (u *Updatable) List(q *http.Request, req *ListRequest, resp *ListResponse) error {
	return u.Metadata.List(q, req, resp)
}

// Get retrieves the value at path given.
func (u *Updatable) Get(q *http.Request, req *GetRequest, resp *GetResponse) error {
	return u.Metadata.Get(q, req, resp)
}

// Changes sends a batch of changes to get a proposed view and cas
func (u *Updatable) Changes(_ *http.Request, req *ChangesRequest, resp *ChangesResponse) error {
	fmt.Println(">>>> changes", req, u.updatable)
	original, proposed, cas, err := u.updatable.Changes(req.Changes)
	resp.Original = original
	resp.Proposed = proposed
	resp.Cas = cas
	return err
}

// Commit commits the changes
func (u *Updatable) Commit(_ *http.Request, req *CommitRequest, resp *CommitResponse) error {
	return u.updatable.Commit(req.Proposed, req.Cas)
}

///////
type updatable struct {
	*client
}

// NewClientUpdatable returns a plugin interface implementation connected to a remote plugin
func NewClientUpdatable(socketPath string) (metadata.Plugin, error) {
	rpcClient, err := rpc_client.New(socketPath, metadata.UpdatableInterfaceSpec)
	if err != nil {
		return nil, err
	}
	return &updatable{client: &client{client: rpcClient}}, nil
}

// AdaptUpdatable converts a rpc client to a Metadata plugin object
func AdaptUpdatable(rpcClient rpc_client.Client) metadata.Updatable {
	return &updatable{client: &client{client: rpcClient}}
}

// List returns a list of nodes under path.
func (u updatable) List(path types.Path) ([]string, error) {
	return u.client.list("Updatable.List", path)
}

// Get retrieves the metadata at path.
func (u updatable) Get(path types.Path) (*types.Any, error) {
	return u.client.get("Updatable.Get", path)
}

// Changes sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
func (u updatable) Changes(changes []metadata.Change) (original, proposed *types.Any, cas string, err error) {
	req := ChangesRequest{Changes: changes}
	resp := ChangesResponse{}
	err = u.client.client.Call("Updatable.Changes", req, &resp)
	return resp.Original, resp.Proposed, resp.Cas, err
}

// Commit asks the plugin to commit the proposed view with the cas.  The cas is used for
// optimistic concurrency control.
func (u updatable) Commit(proposed *types.Any, cas string) error {
	req := CommitRequest{Proposed: proposed, Cas: cas}
	resp := CommitResponse{}
	return u.client.client.Call("Updatable.Commit", req, &resp)
}
