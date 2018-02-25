package resource

import (
	"testing"

	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	"github.com/docker/infrakit/pkg/testing/scope"
	"github.com/stretchr/testify/require"
)

func TestCollection(t *testing.T) {

	c, err := newCollection(
		scope.DefaultScope(),
		resource.Options{})
	require.Error(t, err) // buffer size is 0

	c, err = newCollection(
		scope.DefaultScope(),
		resource.Options{
			LostBufferSize:  100,
			FoundBufferSize: 100,
		})
	require.NoError(t, err)
	require.NotNil(t, c)

}
