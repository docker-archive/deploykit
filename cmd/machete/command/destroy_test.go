package command

import (
	"github.com/docker/libmachete/cmd/machete/console/mock"
	"github.com/docker/libmachete/provisioners"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDestroy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	console := mock.NewMockConsole(ctrl)
	console.EXPECT().Println("I can't do that yet!")

	cmd := destroyCmd(console, &provisioners.Registry{})
	require.Nil(t, cmd.RunE(cmd, []string{}))
}
