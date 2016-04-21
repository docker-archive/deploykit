package libmachete

import (
	"github.com/docker/libmachete/mock"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:generate mockgen -package mock -destination mock/mock_provisioner.go github.com/docker/libmachete/provisioners/api Provisioner

func TestRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	defer clear()
	provisionerA := mock.NewMockProvisioner(ctrl)
	provisionerB := mock.NewMockProvisioner(ctrl)

	params := map[string]string{"a": "b", "c": "d"}

	require.Nil(t, GetProvisioner("nonexistent", params))

	require.Nil(t, RegisterProvisioner("a", func(input map[string]string) api.Provisioner {
		require.Equal(t, params, input)
		return provisionerA
	}))
	require.Error(t, RegisterProvisioner("a", func(input map[string]string) api.Provisioner {
		require.Fail(t, "Duplicate provisioner builder should never be called")
		return nil
	}))

	require.Nil(t, RegisterProvisioner("b", func(input map[string]string) api.Provisioner {
		require.Equal(t, params, input)
		return provisionerB
	}))

	require.Exactly(t, provisionerA, GetProvisioner("a", params))
	require.Exactly(t, provisionerA, GetProvisioner("b", params))
}
