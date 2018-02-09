package group

import (
	"testing"
	"time"

	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestExceedsHealthyThresholdNoNewInsts(t *testing.T) {
	// Count does not matter since there are no instances
	five := 5
	counts := updatingCount{healthyCount: &five}
	require.True(t, counts.exceedsHealthyThreshold(group_types.Updating{Count: 10}, 0))
	require.Equal(t, updatingCount{healthyCount: &five}, counts)
}

func TestExceedsHealthyThresholdDefaults(t *testing.T) {
	// Default (nil) values
	counts := updatingCount{}
	require.True(t, counts.exceedsHealthyThreshold(group_types.Updating{}, 1))
	require.Equal(t, updatingCount{}, counts)

	// Explicit 0 count
	zero := 0
	counts = updatingCount{healthyCount: &zero}
	require.True(t, counts.exceedsHealthyThreshold(group_types.Updating{}, 1))
	require.Equal(t, updatingCount{healthyCount: &zero}, counts)
}

func TestExceedsHealthyThresholdCount(t *testing.T) {
	// Only healthy on the 3 count value
	counts := updatingCount{}
	require.False(t, counts.exceedsHealthyThreshold(group_types.Updating{Count: 3}, 3))
	one := 1
	require.Equal(t, updatingCount{healthyCount: &one}, counts)

	require.False(t, counts.exceedsHealthyThreshold(group_types.Updating{Count: 3}, 3))
	two := 2
	require.Equal(t, updatingCount{healthyCount: &two}, counts)

	require.True(t, counts.exceedsHealthyThreshold(group_types.Updating{Count: 3}, 3))
	three := 3
	require.Equal(t, updatingCount{healthyCount: &three}, counts)
}

func TestExceedsHealthyThresholdDuration(t *testing.T) {
	// Timestamp should be set the first time
	counts := updatingCount{}
	require.False(t, counts.exceedsHealthyThreshold(group_types.Updating{
		Duration: types.FromDuration(3 * time.Second)}, 3))
	require.Nil(t, counts.healthyCount)
	require.NotNil(t, counts.healthyTs)

	// Change the timestamp to be a second before the threshold, should
	// not be healthy
	ts := time.Now().Add(-2 * time.Second)
	counts.healthyTs = &ts
	require.False(t, counts.exceedsHealthyThreshold(group_types.Updating{
		Duration: types.FromDuration(3 * time.Second)}, 3))
	require.Nil(t, counts.healthyCount)
	require.Equal(t, *counts.healthyTs, ts)

	// Change the timestamp to at the threshold, should be
	ts = time.Now().Add(-3 * time.Second)
	counts.healthyTs = &ts
	require.True(t, counts.exceedsHealthyThreshold(group_types.Updating{
		Duration: types.FromDuration(3 * time.Second)}, 3))
	require.Nil(t, counts.healthyCount)
	require.Equal(t, *counts.healthyTs, ts)
}
