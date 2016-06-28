package http

import (
	"github.com/drewolson/testflight"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSwarmLifecycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, handler := prepareTest(t, ctrl)

	testflight.WithServer(handler, func(r *testflight.Requester) {
		// Functions all return 500 as they are not yet implemented.
		response := r.Get("/swarms")
		require.Equal(t, 500, response.StatusCode)

		response = r.Get("/swarms/production")
		require.Equal(t, 500, response.StatusCode)

		response = r.Post("/swarms/production", JSON, "")
		require.Equal(t, 500, response.StatusCode)

		response = r.Delete("/swarms/production", JSON, "")
		require.Equal(t, 500, response.StatusCode)
	})
}
