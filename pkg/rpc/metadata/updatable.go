package metadata

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/rpc/internal"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// UpdatableServerWithNames returns a Metadata that conforms to the net/rpc rpc call convention.
func UpdatableServerWithNames(subplugins func() (map[string]metadata.Plugin, error)) *Updatable {

	keyed := internal.ServeKeyed(
		// This is where templates would be nice...
		func() (map[string]interface{}, error) {
			m, err := subplugins()
			if err != nil {
				return nil, err
			}
			out := map[string]interface{}{}
			for k, v := range m {
				out[string(k)] = v
			}
			return out, nil
		},
	)
	return &Updatable{keyed: keyed}
}

// UpdatableServer returns a Metadata that conforms to the net/rpc rpc call convention.
func UpdatableServer(p metadata.Updatable) *Updatable {
	return &Updatable{keyed: internal.ServeSingle(p)}
}

// Updatable is a rpc object with methods that are exported as JSON RPC service methods
type Updatable struct {
	keyed *internal.Keyed
}

// VendorInfo returns a metadata object about the plugin, if the plugin implements it.  See spi.Vendor
func (u *Updatable) VendorInfo() *spi.VendorInfo {
	base, _ := u.keyed.Keyed(plugin.Name("."))
	if m, is := base.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (u *Updatable) ImplementedInterface() spi.InterfaceSpec {
	return metadata.UpdatableInterfaceSpec
}

// Objects returns the objects exposed by this kind of RPC service
func (u *Updatable) Objects() []rpc.Object {
	return u.keyed.Objects()
}

// Keys returns a list of child nodes given a path.
func (u *Updatable) Keys(_ *http.Request, req *KeysRequest, resp *KeysResponse) error {

	return u.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		nodes, err := v.(metadata.Plugin).Keys(req.Path)
		if err == nil {
			sort.Strings(nodes)
			resp.Nodes = nodes
		}
		return err
	})
}

// Get retrieves the value at path given.
func (u *Updatable) Get(_ *http.Request, req *GetRequest, resp *GetResponse) error {

	return u.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		value, err := v.(metadata.Plugin).Get(req.Path)
		if err == nil {
			resp.Value = value
		}
		return err
	})
}

// Changes sends a batch of changes to get a proposed view and cas
func (u *Updatable) Changes(_ *http.Request, req *ChangesRequest, resp *ChangesResponse) error {

	return u.keyed.Do(req, func(v interface{}) error {

		updater, is := v.(metadata.Updatable)
		if !is {
			return fmt.Errorf("readonly")
		}

		resp.Name = req.Name
		original, proposed, cas, err := updater.Changes(req.Changes)
		resp.Original = original
		resp.Proposed = proposed
		resp.Cas = cas
		return err
	})
}

// Commit commits the changes
func (u *Updatable) Commit(_ *http.Request, req *CommitRequest, resp *CommitResponse) error {

	return u.keyed.Do(req, func(v interface{}) error {

		updater, is := v.(metadata.Updatable)
		if !is {
			return fmt.Errorf("readonly")
		}

		resp.Name = req.Name
		return updater.Commit(req.Proposed, req.Cas)
	})
}

///////
type updatable struct {
	name   plugin.Name
	client rpc_client.Client
}

// NewClientUpdatable returns a plugin interface implementation connected to a remote plugin
func NewClientUpdatable(name plugin.Name, socketPath string) (metadata.Updatable, error) {
	rpcClient, err := rpc_client.New(socketPath, metadata.UpdatableInterfaceSpec)
	if err != nil {
		return nil, err
	}
	return &updatable{name: name, client: rpcClient}, nil
}

// AdaptUpdatable converts a rpc client to a Metadata plugin object
func AdaptUpdatable(name plugin.Name, rpcClient rpc_client.Client) metadata.Updatable {
	return &updatable{name: name, client: rpcClient}
}

// Keys returns a list of nodes under path.
func (u updatable) Keys(path types.Path) ([]string, error) {
	return list(u.name, u.client, "Updatable.Keys", path)
}

// Get retrieves the metadata at path.
func (u updatable) Get(path types.Path) (*types.Any, error) {
	return get(u.name, u.client, "Updatable.Get", path)
}

// Changes sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
func (u updatable) Changes(changes []metadata.Change) (original, proposed *types.Any, cas string, err error) {
	req := ChangesRequest{Name: u.name, Changes: changes}
	resp := ChangesResponse{}
	err = u.client.Call("Updatable.Changes", req, &resp)
	return resp.Original, resp.Proposed, resp.Cas, err
}

// Commit asks the plugin to commit the proposed view with the cas.  The cas is used for
// optimistic concurrency control.
func (u updatable) Commit(proposed *types.Any, cas string) error {
	req := CommitRequest{Name: u.name, Proposed: proposed, Cas: cas}
	resp := CommitResponse{}
	return u.client.Call("Updatable.Commit", req, &resp)
}
