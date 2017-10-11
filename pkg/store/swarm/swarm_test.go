package swarm

import (
	"testing"

	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	mock_client "github.com/docker/infrakit/pkg/mock/docker/docker/client"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var swarmLabel = "infrakit"

var input = map[string]interface{}{
	"Group": map[string]interface{}{
		"managers": map[string]interface{}{
			"Instance":   "foo",
			"Flavor":     "bar",
			"Allocation": []interface{}{"a", "b", "c"},
		},
		"workers": map[string]interface{}{
			"Instance": "bar",
			"Flavor":   "baz",
		},
	},
}

const nodeID = "my-node-id"
const encodedInput = "eJx0jkEKwlAMRPc9xTBrT/B3bhTPIC7STxXx+yNpVVB6dymSGhE3gTeTmeTZAADXptcLE94I8CxVDp31QQO4LEWzDEetTNjOOkDhImL7jZkz7T4GV0VuakxgKxYS3NR+kJq7ydqrenr0Fd7VTj/fxbrHv7rpktc1PsdXAAAA//8CYjjK"

var infoResponse = docker_types.Info{Swarm: swarm.Info{NodeID: nodeID}}

func TestSaveLoadSnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_client.NewMockAPIClientCloser(ctrl)
	snapshot, err := NewSnapshot(client, swarmLabel)

	swarmInfo := swarm.Swarm{}
	expectedSpec := swarm.Spec{
		Annotations: swarm.Annotations{
			Labels: map[string]string{swarmLabel: encodedInput},
		},
	}
	//Test first time save Snapshot (nil -> expectedSpec)

	client.EXPECT().SwarmUpdate(gomock.Any(), swarm.Version{Index: uint64(0)}, expectedSpec, swarm.UpdateFlags{RotateWorkerToken: false, RotateManagerToken: false, RotateManagerUnlockKey: false}).Return(nil)
	client.EXPECT().SwarmInspect(gomock.Any()).Return(swarmInfo, nil)
	err = snapshot.Save(input)
	require.NoError(t, err)

	//Test update Snapshot(unexpectedSpec -> expectedSpec)
	unexpectedSpec := swarm.Spec{
		Annotations: swarm.Annotations{
			Labels: map[string]string{swarmLabel: "dummy"},
		},
	}
	swarmInfo.Spec = unexpectedSpec
	client.EXPECT().SwarmUpdate(gomock.Any(), swarm.Version{Index: uint64(0)}, expectedSpec, swarm.UpdateFlags{RotateWorkerToken: false, RotateManagerToken: false, RotateManagerUnlockKey: false}).Return(nil)
	client.EXPECT().SwarmInspect(gomock.Any()).Return(swarmInfo, nil)
	err = snapshot.Save(input)
	require.NoError(t, err)
	//Test Load Snapshot
	client.EXPECT().SwarmInspect(gomock.Any()).Return(swarmInfo, nil)
	output := map[string]interface{}{}
	err = snapshot.Load(&output)
	require.NoError(t, err)
	require.Equal(t, input, output)
}

func TestEncodeDecode(t *testing.T) {
	encoded, err := encode(input)
	require.NoError(t, err)
	t.Log("encoded=", encoded)

	output := map[string]interface{}{}
	err = decode(encoded, &output)
	require.NoError(t, err)

	require.Equal(t, input, output)
}
