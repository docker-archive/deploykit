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
		scope.FakeLeader(true),
		resource.Options{})
	require.NoError(t, err)
	require.NotNil(t, c.Metadata())

}
