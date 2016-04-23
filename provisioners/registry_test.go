package provisioners

import (
	"errors"
	"github.com/docker/libmachete/provisioners/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:generate mockgen -package mock -destination mock/mock_provisioner.go github.com/docker/libmachete/provisioners/api Provisioner
//go:generate mockgen -package mock -destination mock/mock_creator.go github.com/docker/libmachete/provisioners Creator

func TestRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	params := map[string]string{"a": "b", "c": "d"}

	provisionerA := mock.NewMockProvisioner(ctrl)
	provisionerB := mock.NewMockProvisioner(ctrl)

	creatorA := mock.NewMockCreator(ctrl)
	creatorA.EXPECT().Create(params).AnyTimes().Return(provisionerA, nil)

	creatorB := mock.NewMockCreator(ctrl)
	creatorB.EXPECT().Create(params).AnyTimes().Return(provisionerB, nil)

	creatorE := mock.NewMockCreator(ctrl)
	creatorE.EXPECT().Create(params).AnyTimes().Return(nil, errors.New("nope"))

	registry := NewRegistry(map[string]Creator{
		"a": creatorA,
		"b": creatorB,
		"e": creatorE,
	})

	provisioner, err := registry.Get("nonexistent", params)
	require.NotNil(t, err, "Lookup of a nonexistent provisioner should be an error")
	require.Nil(t, provisioner)

	provisioner, err = registry.Get("a", params)
	require.Nil(t, err)
	require.Exactly(t, provisionerA, provisioner)

	provisioner, err = registry.Get("b", params)
	require.Nil(t, err)
	require.Exactly(t, provisionerB, provisioner)

	provisioner, err = registry.Get("e", params)
	require.NotNil(t, err)
	require.Nil(t, provisioner)
}
