package machines

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/docker/libmachete/api"
	mock_spi "github.com/docker/libmachete/mock/provisioners/spi"
	mock_storage "github.com/docker/libmachete/mock/storage"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/docker/libmachete/storage"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:generate mockgen -package machines -destination ../mock/provisioners/spi/spi.go github.com/docker/libmachete/provisioners/spi Provisioner

func machineRecord(name string) api.MachineRecord {
	return api.MachineRecord{
		MachineSummary: api.MachineSummary{MachineName: api.MachineID(name)},
	}
}

func TestListingSorted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	data := []struct {
		ID     api.MachineID
		Record api.MachineRecord
	}{
		{ID: api.MachineID("host1"), Record: machineRecord("host1")},
		{ID: api.MachineID("host2"), Record: machineRecord("host2")},
		{ID: api.MachineID("host3"), Record: machineRecord("host3")},
	}

	// TODO(wfarner): Consider using a fake rather than mock store, this test is currently highly coupled to
	// implementation details.
	store := mock_storage.NewMockKvStore(ctrl)

	keys := []storage.Key{}
	for _, m := range data {
		keys = append(keys, machineRecordKey(keyFromMachineID(m.ID)))
	}

	store.EXPECT().ListRecursive(recordsRootKey).Return(keys, nil)

	for _, m := range data {
		result := m.Record
		resultJSON, err := json.Marshal(result)
		require.NoError(t, err)
		store.EXPECT().Get(machineRecordKey(keyFromMachineID(m.ID))).Return(resultJSON, nil)
	}

	machines := NewMachines(store)

	summaries, err := machines.List()
	require.NoError(t, err)

	ids := []api.MachineID{}
	for _, summary := range summaries {
		ids = append(ids, summary.MachineName)
	}
	require.Equal(t, []api.MachineID{api.MachineID("host1"), api.MachineID("host2"), api.MachineID("host3")}, ids)

	// Another -- test listing of ids only
	store.EXPECT().ListRecursive(recordsRootKey).Return(keys, nil)

	ids, err = machines.ListIds()
	require.NoError(t, err)
	require.Equal(t, ids, ids)
}

func checkTasksAreRun(t *testing.T, events <-chan interface{}, tasks []spi.Task) {
	seen := []interface{}{}
	// extract the name of the events
	executed := []string{}
	for e := range events {
		seen = append(seen, e)
		if evt, ok := e.(api.Event); ok {
			executed = append(executed, evt.Name)
		}
	}

	require.Equal(t, len(tasks), len(seen))
	taskNames := []string{}
	for _, t := range tasks {
		taskNames = append(taskNames, string(t.Name))
	}
	require.Equal(t, taskNames, executed)
}

func TestDefaultRunCreateInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_spi.NewMockProvisioner(ctrl)
	request := &spi.BaseMachineRequest{}
	record := api.MachineRecord{}

	tasks := []spi.Task{CreateInstance(provisioner)}

	createEvents := make(chan spi.CreateInstanceEvent)
	provisioner.EXPECT().CreateInstance(gomock.Any()).Return(createEvents, nil)
	events, err := runTasks(
		tasks,
		record,
		request,
		func(api.MachineRecord, spi.MachineRequest) error {
			return nil
		},
		func(*api.MachineRecord, spi.MachineRequest) {})
	require.NoError(t, err)
	require.NotNil(t, events)
	close(createEvents) // Simulates the async creation completes
	checkTasksAreRun(t, events, tasks)
}

func TestDefaultRunDestroyInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_spi.NewMockProvisioner(ctrl)
	request := &spi.BaseMachineRequest{}
	record := api.MachineRecord{}

	tasks := []spi.Task{DestroyInstance(provisioner)}

	destroyEvents := make(chan spi.DestroyInstanceEvent)
	provisioner.EXPECT().DestroyInstance(gomock.Any()).Return(destroyEvents, nil)
	events, err := runTasks(
		tasks,
		record,
		request,
		func(api.MachineRecord, spi.MachineRequest) error {
			return nil
		},
		func(*api.MachineRecord, spi.MachineRequest) {})
	require.NoError(t, err)
	require.NotNil(t, events)
	close(destroyEvents) // Simulates the async destroy completes
	checkTasksAreRun(t, events, tasks)
}

func TestCreateMachine(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	name := "new-host"
	id := "id-new-host"

	template := &spi.BaseMachineRequest{Provision: []string{CreateInstanceName}}
	request := new(spi.BaseMachineRequest)
	expectReq := new(spi.BaseMachineRequest)
	*expectReq = *template
	expectReq.MachineName = name

	provisioner := mock_spi.NewMockProvisioner(ctrl)
	provisioner.EXPECT().Name().Return("test-provisioner")
	provisioner.EXPECT().NewRequestInstance().Return(request)

	provision := []spi.Task{CreateInstance(provisioner)}
	provisioner.EXPECT().GetProvisionTasks().Return(provision)
	createEvents := make(chan spi.CreateInstanceEvent)
	provisioner.EXPECT().CreateInstance(expectReq).Return(createEvents, nil)
	provisioner.EXPECT().GetIPAddress(gomock.Any()).Return("ip", nil)
	provisioner.EXPECT().GetInstanceID(gomock.Any()).Return(id, nil)

	store := mock_storage.NewMockKvStore(ctrl)
	store.EXPECT().Save(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)

	record := api.MachineRecord{}
	record.MachineName = api.MachineID(name)
	record.InstanceID = id
	record.AppendChange(&spi.BaseMachineRequest{
		MachineName: string(record.MachineName),
		Provision:   template.Provision,
	})
	notFound := &api.Error{Code: api.ErrNotFound}

	recordJSON, err := json.Marshal(record)
	require.NoError(t, err)

	machineStorageKey := machineRecordKey(keyFromMachineID(record.MachineName))
	store.EXPECT().Get(machineStorageKey).Times(1).Return(recordJSON, notFound)
	store.EXPECT().Get(machineStorageKey).AnyTimes().Return(recordJSON, nil)
	machines := NewMachines(store)
	events, err := machines.CreateMachine(
		provisioner,
		template,
		bytes.NewBuffer([]byte(`{"name": "new-host"}`)),
		api.ContentTypeJSON)
	require.NoError(t, err)
	require.NotNil(t, events)
	close(createEvents) // Simulates the async destroy completes
	checkTasksAreRun(t, events, provision)
}

func TestDestroyMachine(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_spi.NewMockProvisioner(ctrl)

	teardown := []spi.Task{DestroyInstance(provisioner)}
	teardownNames := []string{DestroyInstanceName}

	// Data from previous provisioning run
	record := api.MachineRecord{}
	record.MachineName = "test-host"
	record.InstanceID = "id-test-host"
	record.AppendChange(&spi.BaseMachineRequest{
		MachineName: string(record.MachineName),
		Teardown:    teardownNames,
	})

	provisioner.EXPECT().GetTeardownTasks().Return(teardown)
	destroyEvents := make(chan spi.DestroyInstanceEvent)
	provisioner.EXPECT().DestroyInstance(record.InstanceID).Return(destroyEvents, nil)

	store := mock_storage.NewMockKvStore(ctrl)
	store.EXPECT().Save(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	machines := NewMachines(store)
	events, err := machines.DeleteMachine(provisioner, record)
	require.NoError(t, err)
	require.NotNil(t, events)
	close(destroyEvents) // Simulates the async destroy completes
	checkTasksAreRun(t, events, teardown)
}

func makeTask(name string) spi.Task {
	return spi.Task{
		Name:    name,
		Message: "message",
		Do:      nil,
	}
}

func makeErrorTask(name string) spi.Task {
	return spi.Task{
		Name:    name,
		Message: "message",
		Do: func(spi.Resource, spi.MachineRequest, chan<- interface{}) error {
			return errors.New("test-error")
		},
	}
}

func TestRunTasksNoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	request := &spi.BaseMachineRequest{}
	record := api.MachineRecord{}

	tasks := []spi.Task{
		makeTask("step1"),
		makeTask("step2"),
		makeTask("step3"),
		makeTask("step4"),
	}

	events, err := runTasks(
		tasks,
		record,
		request,
		func(api.MachineRecord, spi.MachineRequest) error {
			return nil
		},
		func(*api.MachineRecord, spi.MachineRequest) {})

	require.NoError(t, err)
	require.NotNil(t, events)

	checkTasksAreRun(t, events, tasks)
}

func TestRunTasksErrorAbort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	request := &spi.BaseMachineRequest{}
	record := api.MachineRecord{}

	tasks := []spi.Task{
		makeTask("step1"),
		makeTask("step2"),
		makeErrorTask("step3"),
		makeTask("step4"),
	}

	events, err := runTasks(
		tasks,
		record,
		request,
		func(api.MachineRecord, spi.MachineRequest) error {
			return nil
		},
		func(*api.MachineRecord, spi.MachineRequest) {})

	require.NoError(t, err)
	require.NotNil(t, events)

	checkTasksAreRun(t, events, tasks[0:3])
}
