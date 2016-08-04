package util

import (
	"github.com/docker/libmachete/spi/instance"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGroupFromRequest(t *testing.T) {
	group, err := GroupFromRequest(`{"Group": "managers"}`)
	require.NoError(t, err)
	require.Equal(t, instance.GroupID("managers"), *group)

	requireFailsWithRequest := func(request string) {
		group, err := GroupFromRequest(request)
		require.Error(t, err)
		require.Nil(t, group)
	}

	requireFailsWithRequest("")
	requireFailsWithRequest("{}")
	requireFailsWithRequest(`{"Group": ""`)
}
