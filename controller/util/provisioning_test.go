package util

import (
	"github.com/docker/libmachete/spi/instance"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGroupFromRequest(t *testing.T) {
	group, _, err := GroupAndCountFromRequest(`{"Group": "managers"}`)
	require.NoError(t, err)
	require.Equal(t, instance.GroupID("managers"), *group)

	requireFailsWithRequest := func(request string) {
		group, _, err := GroupAndCountFromRequest(request)
		require.Error(t, err)
		require.Nil(t, group)
	}

	requireFailsWithRequest("")
	requireFailsWithRequest("{}")
	requireFailsWithRequest(`{"Group": ""`)
}
