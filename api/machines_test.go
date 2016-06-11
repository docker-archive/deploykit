package api

import (
	"bytes"
	"encoding/json"
	"errors"
	mock_spi "github.com/docker/libmachete/mock/provisioners/spi"
	mock_storage "github.com/docker/libmachete/mock/storage"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/docker/libmachete/storage"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:generate mockgen -package api -destination ../mock/provisioners/spi/spi.go github.com/docker/libmachete/provisioners/spi Provisioner

func machineRecord(name string) MachineRecord {
	return MachineRecord{
		MachineSummary: MachineSummary{MachineName: MachineID(name)},
	}
}

func TestListingSorted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	data := []struct {
		ID     MachineID
		Record MachineRecord
	}{
		{ID: MachineID("host1"), Record: machineRecord("host1")},
		{ID: MachineID("host2"), Record: machineRecord("host2")},
		{ID: MachineID("host3"), Record: machineRecord("host3")},
	}

	store := mock_storage.NewMockKvStore(ctrl)

	keys := []storage.Key{}
	for _, m := range data {
		keys = append(keys, keyFromMachineID(m.ID))
	}

	store.EXPECT().ListRecursive(storage.RootKey).Return(keys, nil)

	for _, m := range data {
		result := m.Record
		resultJSON, err := json.Marshal(result)
		require.NoError(t, err)
		store.EXPECT().Get(keyFromMachineID(m.ID)).Return(resultJSON, nil)
	}

	machines := NewMachines(store)

	summaries, err := machines.List()
	require.NoError(t, err)

	ids := []MachineID{}
	for _, summary := range summaries {
		ids = append(ids, summary.MachineName)
	}
	require.Equal(t, []MachineID{MachineID("host1"), MachineID("host2"), MachineID("host3")}, ids)

	// Another -- test listing of ids only
	store.EXPECT().ListRecursive(storage.RootKey).Return(keys, nil)

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
		if evt, ok := e.(Event); ok {
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
	record := MachineRecord{}

	tasks := []spi.Task{CreateInstance(provisioner)}

	createEvents := make(chan spi.CreateInstanceEvent)
	provisioner.EXPECT().CreateInstance(gomock.Any()).Return(createEvents, nil)
	events, err := runTasks(
		tasks,
		record,
		request,
		func(r MachineRecord, q spi.MachineRequest) error {
			return nil
		},
		func(r *MachineRecord, s spi.MachineRequest) {
			return
		})
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
	record := MachineRecord{}

	tasks := []spi.Task{DestroyInstance(provisioner)}

	destroyEvents := make(chan spi.DestroyInstanceEvent)
	provisioner.EXPECT().DestroyInstance(gomock.Any()).Return(destroyEvents, nil)
	events, err := runTasks(
		tasks,
		record,
		request,
		func(r MachineRecord, q spi.MachineRequest) error {
			return nil
		},
		func(r *MachineRecord, s spi.MachineRequest) {
			return
		})
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

	record := new(MachineRecord)
	record.MachineName = MachineID(name)
	record.InstanceID = id
	record.AppendChange(&spi.BaseMachineRequest{
		MachineName: string(record.MachineName),
		Provision:   template.Provision,
	})
	notFound := &Error{Code: ErrNotFound}

	recordJSON, err := json.Marshal(record)
	require.NoError(t, err)

	store.EXPECT().Get(keyFromMachineID(record.MachineName)).Times(1).Return(recordJSON, notFound)
	store.EXPECT().Get(keyFromMachineID(record.MachineName)).AnyTimes().Return(recordJSON, nil)
	machines := NewMachines(store)
	events, err := machines.CreateMachine(
		provisioner,
		template,
		bytes.NewBuffer([]byte(`{"name": "new-host"}`)),
		ContentTypeJSON)
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
	record := new(MachineRecord)
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
	events, err := machines.DeleteMachine(provisioner, *record)
	require.NoError(t, err)
	require.NotNil(t, events)
	close(destroyEvents) // Simulates the async destroy completes
	checkTasksAreRun(t, events, teardown)
}

func makeTask(name string) spi.Task {
	return spi.Task{
		Name:    name,
		Message: "message",
		Do:      func(spi.Resource, spi.MachineRequest, chan<- interface{}) error { return nil },
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
	record := MachineRecord{}

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
		func(r MachineRecord, q spi.MachineRequest) error {
			return nil
		},
		func(r *MachineRecord, s spi.MachineRequest) {
			return
		})

	require.NoError(t, err)
	require.NotNil(t, events)

	checkTasksAreRun(t, events, tasks)
}

func TestRunTasksErrorAbort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	request := &spi.BaseMachineRequest{}
	record := MachineRecord{}

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
		func(r MachineRecord, q spi.MachineRequest) error {
			return nil
		},
		func(r *MachineRecord, s spi.MachineRequest) {
			return
		})

	require.NoError(t, err)
	require.NotNil(t, events)

	checkTasksAreRun(t, events, tasks[0:3])
}
