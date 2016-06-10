package http

import (
	"github.com/docker/libmachete/machines"
	mock_spi "github.com/docker/libmachete/mock/provisioners/spi"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/docker/libmachete/storage/filestore"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

const JSON = "application/json"

func prepareTest(t *testing.T, ctrl *gomock.Controller) (*mock_spi.MockProvisioner, http.Handler) {
	provisioner := mock_spi.NewMockProvisioner(ctrl)

	builder := machines.ProvisionerBuilder{
		Name:                  "testcloud",
		DefaultCredential:     nil,
		DefaultMachineRequest: func() spi.MachineRequest { return &spi.BaseMachineRequest{} },
		Build: func(controls spi.ProvisionControls, cred spi.Credential) (spi.Provisioner, error) {
			return provisioner, nil
		},
	}

	server, err := build(
		filestore.NewFileStore(afero.NewMemMapFs(), "/"),
		machines.NewMachineProvisioners([]machines.ProvisionerBuilder{builder}))
	require.NoError(t, err)

	return provisioner, server.getHandler()
}
