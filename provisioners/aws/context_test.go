package aws

import (
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"testing"
)

func TestWithRegion(t *testing.T) {
	region := "us-west-2"

	_, ok := RegionFromContext(context.Background())
	require.False(t, ok)

	ctx := WithRegion(context.Background(), region)
	found, ok := RegionFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, region, *found)
}

func TestBuildContext(t *testing.T) {

	region := "us-west-2"

	ctx := BuildContext(context.Background(), region)
	require.NotNil(t, ctx)

	r, ok := RegionFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, region, *r)
}
