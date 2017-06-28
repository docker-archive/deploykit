package maxlife

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/spi/instance"
	fake "github.com/docker/infrakit/pkg/testing/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestAge(t *testing.T) {

	link := types.NewLink()
	created := link.Created()

	instance := instance.Description{
		ID:   instance.ID("test"),
		Tags: link.Map(),
	}

	require.Equal(t, 1*time.Hour, age(instance, created.Add(1*time.Hour)))
	require.Equal(t, 59*time.Second, age(instance, created.Add(59*time.Second)))
}

func TestMaxAge(t *testing.T) {

	instances := []instance.Description{}

	for i := 0; i < 3; i++ {
		instances = append(instances, instance.Description{
			ID:   instance.ID(fmt.Sprintf("test%d", i)),
			Tags: types.NewLink().Map(),
		})

		<-time.After(1 * time.Second)
	}

	require.True(t, age(instances[0], time.Now()) > 1*time.Second)
	maxAge := maxAge(instances, time.Now())
	require.Equal(t, "test0", string(maxAge.ID))

}

func TestStartStop(t *testing.T) {

	poll := 100 * time.Millisecond
	maxlife := 1 * time.Second
	tags := map[string]string{}

	plugin := &fake.Plugin{
		DoDescribeInstances: func(tags map[string]string, details bool) ([]instance.Description, error) {
			return nil, nil
		},
		DoDestroy: func(instance instance.ID, ctx instance.Context) error {
			return nil
		},
	}

	controller := NewController("test", plugin, poll, maxlife, tags)
	controller.Start()

	<-time.After(1 * time.Second)

	controller.Stop()
}

func TestEnsureMaxlife(t *testing.T) {

	poll := 100 * time.Millisecond
	maxlife := 1 * time.Second
	tags := map[string]string{}

	all := map[instance.ID]instance.Description{}
	for i := 0; i < 5; i++ {
		inst := instance.Description{
			ID:   instance.ID(fmt.Sprintf("%d", i)),
			Tags: types.NewLink().Map(),
		}
		all[inst.ID] = inst
		<-time.After(500 * time.Millisecond)
	}

	destroy := make(chan instance.ID, 2)
	plugin := &fake.Plugin{
		DoDescribeInstances: func(tags map[string]string, details bool) ([]instance.Description, error) {
			list := []instance.Description{}
			// Return instances in sorted order by instance ID
			keys := make([]string, len(all))
			i := 0
			for key := range all {
				keys[i] = string(key)
				i++
			}
			sort.Strings(keys)
			for _, key := range keys {
				list = append(list, all[instance.ID(key)])
			}
			return list, nil
		},
		DoDestroy: func(instance instance.ID, ctx instance.Context) error {
			delete(all, instance)
			destroy <- instance
			return nil
		},
	}

	controller := NewController("test", plugin, poll, maxlife, tags)

	go controller.ensureMaxlife(len(all))

	<-time.After(2 * time.Second)
	controller.Stop()

	// now read what we were destroying
	d := <-destroy

	require.Equal(t, instance.ID("0"), d)

}
