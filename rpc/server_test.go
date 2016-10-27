package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	plugin_mock "github.com/docker/infrakit/mock/spi/instance"
	plugin_rpc "github.com/docker/infrakit/rpc/instance"
	"github.com/docker/infrakit/spi/instance"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestUnixSocketServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := plugin_mock.NewMockPlugin(ctrl)

	instanceID := instance.ID("id")
	spec := instance.Spec{
		Tags: map[string]string{
			"tag1": "value1",
		},
		Init: "init",
	}

	properties := json.RawMessage([]byte(`{"foo":"bar"}`))
	validateErr := errors.New("validate-error")

	gomock.InOrder(
		mock.EXPECT().Validate(properties).Return(validateErr),
		mock.EXPECT().Provision(spec).Return(&instanceID, nil),
	)

	service := plugin_rpc.PluginServer(mock)

	socket := filepath.Join(os.TempDir(), fmt.Sprintf("%d.sock", time.Now().Unix()))
	stop, errors, err := StartPluginAtPath(socket, service)
	require.NoError(t, err)

	c, err := plugin_rpc.NewClient("unix", socket)
	require.NoError(t, err)

	err = c.Validate(properties)
	require.Error(t, err)
	require.Equal(t, validateErr.Error(), err.Error())

	id, err := c.Provision(spec)
	require.NoError(t, err)
	require.Equal(t, instanceID, *id)

	// Now we stop the server
	close(stop)

	// We shouldn't block here.
	<-errors
}
