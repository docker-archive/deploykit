package event

import (
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestTopic(t *testing.T) {
	parent := Topic("parent")
	child := Topic("child").Under(parent)
	require.Equal(t, "parent/child", child.String())
}

func TestSortTopic(t *testing.T) {

	list := []Topic{
		NewTopic("foo"),
		NewTopic("bar"),
		NewTopic("ace"),
	}

	Sort(list)
	require.Equal(t, []Topic{NewTopic("ace"), NewTopic("bar"), NewTopic("foo")}, list)
}

func TestEventBuilder(t *testing.T) {

	event := Event{
		ID:    "myhost",
		Topic: NewTopic("instance/create"),
	}.Init().WithDataMust([]int{1, 2}).Now()

	require.Equal(t, "myhost", event.ID)
	require.Equal(t, NewTopic("instance/create"), event.Topic)

	event2 := event.Init().WithTopic("instance/delete")
	require.Equal(t, event.ID, event2.ID)
	require.Equal(t, event.Data, event2.Data)
	require.Equal(t, event.Timestamp, event2.Timestamp)

	event.WithData(map[string]bool{"foo": true})
	require.NotEqual(t, event.Data, event2.Data)
}

func TestEncodeDecode(t *testing.T) {

	event := (&Event{
		Topic: Topic("cluster/instance/compute/node1"),
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
	require.Equal(t, event.Timestamp, event2.Timestamp)
	require.Equal(t, event.Data, event2.Data)
	require.Equal(t, any.Bytes(), types.AnyValueMust(event2).Bytes())

	event3 := (&Event{ID: "event3", Topic: Topic("foo")}).Now().WithDataMust([]int{1, 2}).ReceivedNow()
	any3 := types.AnyValueMust(event3)

	event4 := Event{
		ID:    "event4",
		Topic: NewTopic("bar"),
	}.Init().FromAny(any3)

	require.Equal(t, event3, event4) // completely overwritten
}
