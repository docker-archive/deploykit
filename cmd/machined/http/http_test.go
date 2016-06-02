package http

import (
	"github.com/docker/libmachete"
	mock_api "github.com/docker/libmachete/mock/provisioners/api"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage/filestores"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

const JSON = "application/json"

func prepareTest(t *testing.T, ctrl *gomock.Controller) (*mock_api.MockProvisioner, http.Handler) {
	provisioner := mock_api.NewMockProvisioner(ctrl)

	builder := libmachete.ProvisionerBuilder{
		Name:                  "testcloud",
		DefaultCredential:     nil,
		DefaultMachineRequest: func() api.MachineRequest { return &api.BaseMachineRequest{} },
		Build: func(controls api.ProvisionControls, cred api.Credential) (api.Provisioner, error) {
			return provisioner, nil
		},
	}

	sandbox := filestores.NewSandbox(afero.NewMemMapFs(), "/")

	server, err := build(sandbox, libmachete.NewMachineProvisioners([]libmachete.ProvisionerBuilder{builder}))
	require.NoError(t, err)

	return provisioner, server.getHandler()
}
