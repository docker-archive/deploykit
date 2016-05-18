package libmachete

import (
	"github.com/docker/libmachete/provisioners/api"
	"golang.org/x/net/context"
)

// ProvisionerBuilder constructs an instance of provisioner given the context and credential.
type ProvisionerBuilder func(context.Context, api.Credential) (api.Provisioner, error)

var (
	provisionerBuilders = map[string]ProvisionerBuilder{}
)

// RegisterProvisionerBuilder is called in the init() of the provisioner package to register provisioner implementation
func RegisterProvisionerBuilder(provisionerName string, builder ProvisionerBuilder) {
	lock.Lock()
	defer lock.Unlock()
	provisionerBuilders[provisionerName] = builder
}

// GetProvisioner returns an instance of a provisioner identified by name and for the running context and credential
func GetProvisioner(provisionerName string, ctx context.Context, cred api.Credential) (api.Provisioner, error) {
	if builder, has := provisionerBuilders[provisionerName]; has {
		return builder(ctx, cred)
	}
	return nil, &Error{Code: ErrNotFound, Message: "no such provisioner " + provisionerName}
}
