package command

import (
	mock_console "github.com/docker/libmachete/cmd/machete/console/mock"
	"github.com/docker/libmachete/mock"
	"github.com/docker/libmachete/provisioners"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCreateBadUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	console := mock_console.NewMockConsole(ctrl)
	templates := mock.NewMockTemplates(ctrl)

	cmd := createCmd(console, &provisioners.Registry{}, templates)
	require.Exactly(t, UsageError, cmd.RunE(cmd, []string{}))
}
