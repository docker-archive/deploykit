package instance

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/instance"
	. "github.com/docker/infrakit/pkg/testing"
	testing_instance "github.com/docker/infrakit/pkg/testing/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestTracker(t *testing.T) {

	steps := make(chan int, 1)
	describes := make(chan []instance.Description, 1)

	inst1 := &testing_instance.Plugin{

		DoDescribeInstances: func(tags map[string]string, details bool) ([]instance.Description, error) {

			for step := range steps {

				switch step {
				case 0:
					return <-describes, nil
				case 1:
					return <-describes, nil
				case 2:
					return <-describes, nil
				case 3:
					return nil, fmt.Errorf("error")
				case 4:
					return <-describes, nil
				case 5:
					return <-describes, nil
				}
			}
			return nil, nil
		},
	}

	tick := make(chan time.Time)

	m := NewTracker("inst1", inst1, tick, nil)

	chanEvents := make(chan *event.Event, 100)

	m.PublishOn(chanEvents)

	// Send different responses each step

	// initial state - found = 1
	steps <- 0
	describes <- []instance.Description{
		{
			ID: instance.ID("a"),
		},
	}
	tick <- time.Now()

	// initial state - found = 2
	steps <- 0
	describes <- []instance.Description{
		{
			ID: instance.ID("a"),
		},
		{
			ID: instance.ID("b"),
		},
	}
	tick <- time.Now()

	// lost 1, gain 1 = found = 1; lost = 1
	steps <- 1
	describes <- []instance.Description{
		{
			ID: instance.ID("b"),
		},
		{
			ID: instance.ID("c"),
		},
	}
	tick <- time.Now()

	// steady state
	steps <- 2
	describes <- []instance.Description{
		{
			ID: instance.ID("b"),
		},
		{
			ID: instance.ID("c"),
		},
	}
	tick <- time.Now()

	// error error = 1
	steps <- 3
	tick <- time.Now()

	// another - now recovered
	steps <- 4
	describes <- []instance.Description{
		{
			ID: instance.ID("b"),
		},
		{
			ID: instance.ID("c"),
		},
	}
	tick <- time.Now()

	// found 1 more
	steps <- 5
	describes <- []instance.Description{
		{
			ID: instance.ID("b"),
		},
		{
			ID: instance.ID("c"),
		},
		{
			ID: instance.ID("d"),
		},
	}
	tick <- time.Now()

	// shutdown after a wait
	<-time.After(100 * time.Millisecond)
	m.Stop()

	T(100).Infoln("checking events")
	events := []*event.Event{}
	for event := range chanEvents {
		events = append(events, event)
	}

	require.Equal(t, 6, len(events))

	checkEvent(t, "a", "found", events[0])
	checkEvent(t, "b", "found", events[1])
	checkEvent(t, "a", "lost", events[2])
	checkEvent(t, "c", "found", events[3])
	checkEvent(t, "error", "error", events[4])
	checkEvent(t, "d", "found", events[5])

}

func checkEvent(t *testing.T, id string, topic string, event *event.Event) {
	require.Equal(t, id, event.ID)
	require.Equal(t, types.PathFromString(topic), event.Topic)
}
