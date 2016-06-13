package http

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/drewolson/testflight"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func requireMachines(t *testing.T, r *testflight.Requester, expected ...api.MachineID) {
	response := r.Get("/machines?long=true")
	require.Equal(t, 200, response.StatusCode, response.Body)
	require.Equal(t, JSON, response.Header.Get("Content-Type"))
	if expected == nil {
		expected = []api.MachineID{}
	}

	summaries := []api.MachineSummary{}
	err := json.Unmarshal([]byte(response.Body), &summaries)
	require.NoError(t, err)

	actualIds := []api.MachineID{}
	for _, summary := range summaries {
		actualIds = append(actualIds, summary.MachineName)
	}

	require.Equal(t, expected, actualIds)

	// Also validate the short form
	response = r.Get("/machines")
	require.Equal(t, 200, response.StatusCode, response.Body)
	require.Equal(t, JSON, response.Header.Get("Content-Type"))

	requireUnmarshalEqual(t, &expected, response.Body, &[]api.MachineID{})
}

func fetchRecord(t *testing.T, r *testflight.Requester, machineName string) api.MachineRecord {
	response := r.Get(fmt.Sprintf("/machines/%s", machineName))
	require.Equal(t, 200, response.StatusCode, response.Body)
	record := api.MachineRecord{}
	err := json.Unmarshal([]byte(response.Body), &record)
	require.NoError(t, err)
	return record
}

func TestMachineLifecycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvisioner, handler := prepareTest(t, ctrl)

	testflight.WithServer(handler, func(r *testflight.Requester) {
		// There should initially be no machines
		requireMachines(t, r)

		// Add a template for a machine
		template := testMachineRequest{
			BaseMachineRequest: spi.BaseMachineRequest{
				Provisioner: "testcloud",
				Teardown:    []string{api.DestroyInstanceName},
			},
			TurboButtons: 5,
		}
		response := r.Post("/templates/testcloud/prodtemplate", JSON, requireMarshalSuccess(t, template))
		require.Equal(t, 200, response.StatusCode)

		// Add credentials for the provisioner
		response = r.Post(
			"/credentials/testcloud/production",
			JSON,
			requireMarshalSuccess(t, testCredentials{Identity: "larry", Secret: "12345"}))
		require.Equal(t, 200, response.StatusCode)

		// Also register default credentials for the provisioner
		response = r.Post(
			"/credentials/testcloud/default",
			JSON,
			requireMarshalSuccess(t, testCredentials{Identity: "root", Secret: "password"}))
		require.Equal(t, 200, response.StatusCode)

		// Create a machine
		id := api.MachineID("frontend")
		mockProvisioner.EXPECT().NewRequestInstance().Return(&testMachineRequest{})
		mockProvisioner.EXPECT().Name().Return("testcloud")
		mockProvisioner.EXPECT().GetProvisionTasks().Return([]spi.Task{api.CreateInstance(mockProvisioner)})
		expectedRequest := testMachineRequest{
			BaseMachineRequest: spi.BaseMachineRequest{
				MachineName: string(id),
				Provisioner: "testcloud",
				Provision:   []string{api.CreateInstanceName},
				Teardown:    []string{api.DestroyInstanceName},
			},
			TurboButtons: 6,
		}
		createEvents := make(chan spi.CreateInstanceEvent)
		mockProvisioner.EXPECT().CreateInstance(&expectedRequest).Return(createEvents, nil)
		go func() {
			createEvents <- spi.CreateInstanceEvent{}
			close(createEvents)
		}()
		mockProvisioner.EXPECT().GetIPAddress(&expectedRequest).Return("127.0.0.1", nil)
		instanceID := "instance-1"
		mockProvisioner.EXPECT().GetInstanceID(&expectedRequest).Return(instanceID, nil)

		overlay := testMachineRequest{
			BaseMachineRequest: spi.BaseMachineRequest{
				MachineName: string(id),
				Provision:   []string{api.CreateInstanceName},
			},
			TurboButtons: 6,
		}

		response = r.Post(
			"/machines/frontend"+
				"?provisioner=testcloud&credentials=production&template=prodtemplate&block=true",
			JSON,
			requireMarshalSuccess(t, overlay))
		require.Equal(t, 200, response.StatusCode, response.Body)

		// It should return by id
		record := fetchRecord(t, r, "frontend")
		require.Equal(t, id, record.MachineName)
		require.Equal(t, "testcloud", record.Provisioner)
		require.Equal(t, "provisioned", record.Status)
		require.Equal(t, instanceID, record.InstanceID)

		// It should appear in a list request
		requireMachines(t, r, id)

		// It should still appear in a list request
		requireMachines(t, r, id)

		// Delete the machine
		mockProvisioner.EXPECT().GetTeardownTasks().Return([]spi.Task{api.DestroyInstance(mockProvisioner)})
		destroyEvents := make(chan spi.DestroyInstanceEvent)
		mockProvisioner.EXPECT().DestroyInstance(instanceID).Return(destroyEvents, nil)
		go func() {
			destroyEvents <- spi.DestroyInstanceEvent{}
			close(destroyEvents)
		}()

		require.Equal(t, 200, r.Delete("/machines/frontend?block=true", JSON, "").StatusCode)

		// It should be moved to the destroyed state.
		record = fetchRecord(t, r, "frontend")
		require.Equal(t, id, record.MachineName)
		require.Equal(t, "testcloud", record.Provisioner)
		require.Equal(t, "terminated", record.Status)
		require.Equal(t, instanceID, record.InstanceID)
	})
}
