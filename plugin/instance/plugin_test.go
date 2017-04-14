package instance

import (
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	plugin := &plugin{}
	err := plugin.Validate(types.AnyString(`{"Size":"1gb", "Image": "debian-8-x64"}`))

	require.NoError(t, err)
}

func TestValidateFails(t *testing.T) {
	plugin := &plugin{}
	err := plugin.Validate(types.AnyString("-"))

	require.Error(t, err)
}
