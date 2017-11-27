package rpc

import (
	"testing"

	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecode(t *testing.T) {

	objects := map[spi.InterfaceSpec]func() []Object{
		{
			Name:    "test",
			Version: "1.0",
		}: func() []Object {
			return []Object{
				{
					Name: "a",
				},
				{
					Name: "b",
				},
			}
		},
		{
			Name:    "compute",
			Version: "1.1",
		}: func() []Object {
			return []Object{
				{
					Name: "x",
				},
				{
					Name: "y",
				},
			}
		},
	}

	resp := HelloResponse{}

	err := Handshake(objects).Hello(nil, nil, &resp)
	require.NoError(t, err)

	any := types.AnyValueMust(resp)
	buff, err := any.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, `{
"Objects": {
"compute/1.1": [
{
"Name": "x",
"ProxyFor": ""
},
{
"Name": "y",
"ProxyFor": ""
}
],
"test/1.0": [
{
"Name": "a",
"ProxyFor": ""
},
{
"Name": "b",
"ProxyFor": ""
}
]
}
}`, string(buff))

}
