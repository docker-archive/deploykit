package http

import (
	"encoding/json"
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

//go:generate mockgen -package api -destination ../../../mock/provisioners/spi/spi.go github.com/docker/libmachete/provisioners/spi Provisioner

const JSON = "application/json"

type testCredentials struct {
	spi.CredentialBase
	Identity string `json:"identity"`
	Secret   string `json:"secret"`
}

type testMachineRequest struct {
	spi.BaseMachineRequest `yaml:",inline"`
	Quantum                bool `yaml:"quantum,omitempty"`
	TurboButtons           uint `yaml:"turbo_buttons,omitempty"`
}

func prepareTest(t *testing.T, ctrl *gomock.Controller) (*mock_spi.MockProvisioner, http.Handler) {
	provisioner := mock_spi.NewMockProvisioner(ctrl)

	builder := machines.ProvisionerBuilder{
		Name:                  "testcloud",
		DefaultCredential:     func() spi.Credential { return &testCredentials{} },
		DefaultMachineRequest: func() spi.MachineRequest { return &testMachineRequest{} },
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

func requireMarshalSuccess(t *testing.T, entity interface{}) string {
	body, err := json.Marshal(entity)
	require.NoError(t, err)
	return string(body)
}

func requireUnmarshalEqual(t *testing.T, expected interface{}, data string, value interface{}) {
	err := json.Unmarshal([]byte(data), &value)
	require.NoError(t, err)
	require.Equal(t, expected, value)
}
