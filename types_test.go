package libmachete

import (
	_ "encoding/json"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"testing"
	"time"
)

func TestAppendChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	name := "test-host"
	provisionerName := "test"
	version := "0.1"
	provision := []api.TaskName{
		api.TaskName("p1"),
		api.TaskName("p2"),
		api.TaskName("p3"),
	}
	teardown := []api.TaskName{
		api.TaskName("t1"),
		api.TaskName("t2"),
		api.TaskName("t3"),
	}
	change := api.BaseMachineRequest{
		MachineName:        name,
		Provisioner:        provisionerName,
		ProvisionerVersion: version,
		Provision:          provision,
		Teardown:           teardown,
	}

	record := &MachineRecord{
		MachineSummary: MachineSummary{
			MachineName:  MachineID("test-host"),
			Created:      Timestamp(time.Now().Unix()),
			LastModified: Timestamp(time.Now().Unix()),
		},
	}

	record.AppendChange(change)

	require.Equal(t, 1, len(record.Changes))
	require.Equal(t, name, record.GetLastChange().Name())
	require.Equal(t, provisionerName, record.GetLastChange().ProvisionerName())
	require.Equal(t, version, record.GetLastChange().Version())
	require.Equal(t, provision, record.GetLastChange().ProvisionWorkflow())
	require.Equal(t, teardown, record.GetLastChange().TeardownWorkflow())
}

func TestMarshalMachineRecord(t *testing.T) {
	record := &MachineRecord{
		MachineSummary: MachineSummary{
			MachineName:  MachineID("test-host"),
			Created:      Timestamp(time.Now().Unix()),
			LastModified: Timestamp(time.Now().Unix()),
		},
		Changes: []*api.BaseMachineRequest{
			{
				MachineName: "test-host",
				Provisioner: "test",
				Provision: []api.TaskName{
					api.TaskName("task1"),
					api.TaskName("task2"),
					api.TaskName("task3"),
				},
				Teardown: []api.TaskName{
					api.TaskName("task1"),
					api.TaskName("task2"),
					api.TaskName("task3"),
				},
			},
		},
	}

	buff, err := yaml.Marshal(record)
	require.NoError(t, err)
	require.True(t, len(buff) > 0)
}

func TestUnmarshalMachineRecord(t *testing.T) {
	input := `
name: test-host
provisioner: test
created: 1464122054
modified: 1464122054
events: []
changes:
- name: test-host
  provisioner: test
  version: "0.1"
  provision:
  - task1
  - task2
  - task3
  teardown:
  - task1
  - task2
  - task3
`
	record := new(MachineRecord)

	err := yaml.Unmarshal([]byte(input), record)
	require.NoError(t, err)

	require.Equal(t, 1, len(record.Changes))
	require.Equal(t, 3, len(record.GetLastChange().ProvisionWorkflow()))
	require.Equal(t, 3, len(record.GetLastChange().TeardownWorkflow()))
}
