package event

import (
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestEventBuilder(t *testing.T) {

	event := Event{
		ID:    "myhost",
		Topic: types.PathFromString("instance/create"),
	}.Init().WithDataMust([]int{1, 2}).Now()

	require.Equal(t, "myhost", event.ID)
	require.Equal(t, types.PathFromString("instance/create"), event.Topic)

	<-time.After(10 * time.Millisecond) // note that this will force a later time stamp when init by copy
	event2 := event.Init().WithTopic("instance/delete")
	require.Equal(t, event.ID, event2.ID)
	require.Equal(t, event.Data, event2.Data)
	require.True(t, event.Timestamp.Before(event2.Timestamp))

	event.WithData(map[string]bool{"foo": true})
	require.NotEqual(t, event.Data, event2.Data)
}

func TestEncodeDecode(t *testing.T) {

	event := (&Event{
		Topic: types.PathFromString("cluster/instance/compute/node1"),
		Type:  Type("instance_create"),
		ID:    "node1",
		Data:  types.AnyValueMust(map[string]interface{}{"foo": "bar"}),
	}).Now()

	any := types.AnyValueMust(event)
	require.NotNil(t, any)

	any2 := types.AnyBytes(any.Bytes())

	event2 := Event{}
	err := any2.Decode(&event2)
	require.NoError(t, err)
	require.Equal(t, event.Timestamp.Unix(), event2.Timestamp.Unix())
	require.Equal(t, event.Data, event2.Data)
	require.Equal(t, any.Bytes(), types.AnyValueMust(event2).Bytes())

	event3 := (&Event{ID: "event3", Topic: types.PathFromString("foo")}).Now().WithDataMust([]int{1, 2}).ReceivedNow()
	any3 := types.AnyValueMust(event3)

	event4 := Event{
		ID:    "event4",
		Topic: types.PathFromString("bar"),
	}.Init().FromAny(any3)

	// completely overwritten -- on linux there are problems with Equal for time.Time so compare strings
	require.Equal(t, types.AnyValueMust(event3).String(), types.AnyValueMust(event4).String())

	// get bytes
	a, err := event3.Bytes()
	require.NoError(t, err)
	b, err := event4.Bytes()
	require.NoError(t, err)
	require.Equal(t, a, b)
}
