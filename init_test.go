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

	registry := newEmptyRegistry()
	provisionerA := mock.NewMockProvisioner(ctrl)
	provisionerB := mock.NewMockProvisioner(ctrl)

	params := map[string]string{"a": "b", "c": "d"}

	require.Nil(t, registry.Get("nonexistent", params))

	require.Nil(t, registry.Register("a", func(input map[string]string) api.Provisioner {
		require.Equal(t, params, input)
		return provisionerA
	}))
	require.Error(t, registry.Register("a", func(input map[string]string) api.Provisioner {
		require.Fail(t, "Duplicate provisioner builder should never be called")
		return nil
	}))

	require.Nil(t, registry.Register("b", func(input map[string]string) api.Provisioner {
		require.Equal(t, params, input)
		return provisionerB
	}))

	require.Exactly(t, provisionerA, registry.Get("a", params))
	require.Exactly(t, provisionerA, registry.Get("b", params))
}

func TestGetGlobalRegistry(t *testing.T) {
	require.Exactly(t, GetGlobalRegistry(), GetGlobalRegistry())
}
