package api

import (
	"errors"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/stretchr/testify/require"
	"testing"
)

func makeTask(name string) spi.Task {
	return spi.Task{
		Name:    spi.TaskName(name),
		Message: "message",
		Do:      nil,
	}
}

func makeErrorTask(name string) spi.Task {
	return spi.Task{
		Name:    spi.TaskName(name),
		Message: "message",
		Do: func(spi.Provisioner, spi.KeyStore, spi.Credential, spi.Resource, spi.MachineRequest, chan<- interface{}) error {
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

	require.Equal(t, []spi.TaskName{a.Name, b.Name, c.Name}, taskMap.Names())

	_, err := taskMap.Filter([]spi.TaskName{spi.TaskName("d")})
	require.Error(t, err)

	filtered, err := taskMap.Filter([]spi.TaskName{a.Name, c.Name})
	require.NoError(t, err)
	require.Equal(t, []spi.Task{a, c}, filtered)
}
