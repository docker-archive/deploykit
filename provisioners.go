package libmachete

import (
	"github.com/docker/libmachete/provisioners/api"
	"golang.org/x/net/context"
	"sync"
)

// ProvisionerBuilder defines structures needed for the provisioner to operate, and is capable
// of constructing a provisioner.
type ProvisionerBuilder struct {
	Name                  string
	DefaultCredential     api.Credential
	DefaultMachineRequest api.MachineRequest
	BuildContext          ContextBuilder
	Build                 func(ctx context.Context, cred api.Credential) (api.Provisioner, error)
}

var (
	buildersLock = sync.Mutex{}
	builders     = map[string]ProvisionerBuilder{}
)

// RegisterProvisioner makes a provisioner available for other components to fetch.
func RegisterProvisioner(builder ProvisionerBuilder) {
	buildersLock.Lock()
	defer buildersLock.Unlock()

	builders[builder.Name] = builder
}

// GetProvisionerBuilder fetches a provisioner builder by name.
func GetProvisionerBuilder(name string) (ProvisionerBuilder, bool) {
	builder, has := builders[name]
	return builder, has
}

// GetProvisioner returns an instance of a provisioner identified by name and for the running
// context and credential.
func GetProvisioner(name string, ctx context.Context, cred api.Credential) (api.Provisioner, error) {
	if builder, has := builders[name]; has {
		return builder.Build(ctx, cred)
	}
	return nil, &Error{Code: ErrNotFound, Message: "no such provisioner " + name}
}
