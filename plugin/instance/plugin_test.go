package instance

import (
	"testing"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestLabels(t *testing.T) {
	plugin := &plugin{
		tags: &fakeTagsService{},
	}
	id := instance.ID("foo")
	err := plugin.Label(id, map[string]string{
		"foo":    "bar",
		"banana": "baz",
	})

	require.NoError(t, err)
}

func TestLabelFails(t *testing.T) {
	plugin := &plugin{
		tags: &fakeTagsService{
			expectedErr: "something went wrong",
		},
	}
	id := instance.ID("foo")
	err := plugin.Label(id, map[string]string{
		"foo": "bar",
	})

	require.Error(t, err)
}

func TestValidate(t *testing.T) {
	plugin := &plugin{}
	err := plugin.Validate(types.AnyString(`{"Size":"1gb", "Image": "debian-8-x64"}`))

	require.NoError(t, err)
}

func TestValidateFails(t *testing.T) {
	plugin := &plugin{}
	err := plugin.Validate(types.AnyString("-"))

	require.Error(t, err)
}

func TestDestroyFails(t *testing.T) {
	plugin := &plugin{
		droplets: &fakeDropletsServices{
			expectedErr: "something went wrong",
		},
	}
	id := instance.ID("foo")
	err := plugin.Destroy(id)

	require.EqualError(t, err, "strconv.Atoi: parsing \"foo\": invalid syntax")

	id = instance.ID("12345")
	err = plugin.Destroy(id)

	require.EqualError(t, err, "something went wrong")
}

func TestDestroy(t *testing.T) {
	// FIXME(vdemeester) make a better test :D
	plugin := &plugin{
		droplets: &fakeDropletsServices{},
	}
	id := instance.ID("12345")
	err := plugin.Destroy(id)

	require.NoError(t, err)
}
