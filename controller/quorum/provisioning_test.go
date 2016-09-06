package quorum

import (
	"github.com/docker/libmachete/spi/group"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGroupFromRequest(t *testing.T) {
	gid, _, err := groupAndCountFromRequest(`{"Group": "managers"}`)
	require.NoError(t, err)
	require.Equal(t, group.ID("managers"), *gid)

	requireFailsWithRequest := func(request string) {
		group, _, err := groupAndCountFromRequest(request)
		require.Error(t, err)
		require.Nil(t, group)
	}

	requireFailsWithRequest("")
	requireFailsWithRequest("{}")
	requireFailsWithRequest(`{"Group": ""`)
}
