package libmachete

import (
	"errors"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"testing"
)

func makeTask(name string) api.Task {
	return api.Task{
		Name:    api.TaskName(name),
		Message: "message",
		Do:      nil,
	}
}

func makeErrorTask(name string) api.Task {
	return api.Task{
		Name:    api.TaskName(name),
		Message: "message",
		Do: func(api.Provisioner, context.Context,
			api.Credential, api.Resource, api.MachineRequest, chan<- interface{}) error {
			return errors.New("test-error")
		},
	}
}

func TestTaskMap(t *testing.T) {
	a := makeTask("a")
	b := makeTask("b")
	c := makeTask("c")

	require.Panics(t, func() {
		NewTaskMap(a, a)
	})

	taskMap := NewTaskMap(a, b, c)

	require.Equal(t, []api.TaskName{a.Name, b.Name, c.Name}, taskMap.Names())

	_, err := taskMap.Filter([]api.TaskName{api.TaskName("d")})
	require.Error(t, err)

	filtered, err := taskMap.Filter([]api.TaskName{a.Name, c.Name})
	require.NoError(t, err)
	require.Equal(t, []api.Task{a, c}, filtered)
}
