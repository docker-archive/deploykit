package libmachete

import (
	"bytes"
	mock_api "github.com/docker/libmachete/mock/provisioners/api"
	mock_storage "github.com/docker/libmachete/mock/storage"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:generate mockgen -package storage -destination mock/storage/machines.go github.com/docker/libmachete/storage Machines
//go:generate mockgen -package api -destination mock/provisioners/api/provisioner.go github.com/docker/libmachete/provisioners/api Provisioner
//go:generate mockgen -package api -destination mock/provisioners/api/credential.go github.com/docker/libmachete/provisioners/api Credential

func machineRecord(name string) storage.MachineRecord {
	return storage.MachineRecord{
		MachineSummary: storage.MachineSummary{
			MachineName: storage.MachineID(name),
		},
	}
}

func TestListingSorted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	data := []struct {
		ID     storage.MachineID
		Record storage.MachineRecord
	}{
		{ID: storage.MachineID("host2"), Record: machineRecord("host2")},
		{ID: storage.MachineID("host1"), Record: machineRecord("host1")},
		{ID: storage.MachineID("host3"), Record: machineRecord("host3")},
	}

	store := mock_storage.NewMockMachines(ctrl)

	ids := []storage.MachineID{}
	for _, m := range data {
		ids = append(ids, m.ID)
	}

	store.EXPECT().List().Return(ids, nil)

	for _, m := range data {
		result := m.Record
		store.EXPECT().GetRecord(m.ID).Return(&result, nil)
	}

	machines := NewMachines(store)

	summaries, err := machines.List()
	require.NoError(t, err)

	names := []string{}
	for _, s := range summaries {
		names = append(names, string(s.MachineName))
	}
	require.Equal(t, []string{"host1", "host2", "host3"}, names)

	// Another -- test listing of ids only
	store.EXPECT().List().Return(ids, nil)

	l, err := machines.ListIds()
	require.NoError(t, err)
	require.Equal(t, names, l)
}

func checkTasksAreRun(t *testing.T, events <-chan interface{}, tasks []api.Task) {
	seen := []interface{}{}
	// extract the name of the events
	executed := []string{}
	for e := range events {
		seen = append(seen, e)
		if evt, ok := e.(storage.Event); ok {
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

	provisioner := mock_api.NewMockProvisioner(ctrl)
	cred := mock_api.NewMockCredential(ctrl)
	request := &api.BaseMachineRequest{}
	record := new(storage.MachineRecord)

	tasks := []api.Task{
		TaskCreateInstance,
	}

	createEvents := make(chan api.CreateInstanceEvent)
	provisioner.EXPECT().CreateInstance(gomock.Any()).Return(createEvents, nil)
	events, err := runTasks(
		provisioner,
		tasks,
		record,
		cred,
		request,
		func(r storage.MachineRecord, q api.MachineRequest) error {
			return nil
		},
		func(r *storage.MachineRecord, s api.MachineRequest) {
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

	provisioner := mock_api.NewMockProvisioner(ctrl)
	cred := mock_api.NewMockCredential(ctrl)
	request := &api.BaseMachineRequest{}
	record := new(storage.MachineRecord)

	tasks := []api.Task{
		TaskDestroyInstance,
	}

	destroyEvents := make(chan api.DestroyInstanceEvent)
	provisioner.EXPECT().DestroyInstance(gomock.Any()).Return(destroyEvents, nil)
	events, err := runTasks(
		provisioner,
		tasks,
		record,
		cred,
		request,
		func(r storage.MachineRecord, q api.MachineRequest) error {
			return nil
		},
		func(r *storage.MachineRecord, s api.MachineRequest) {
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

	provisioner := mock_api.NewMockProvisioner(ctrl)
	cred := mock_api.NewMockCredential(ctrl)

	provision := []api.Task{
		TaskSSHKeyGen,
		TaskCreateInstance,
	}
	provisionNames := []api.TaskName{}
	for _, t := range provision {
		provisionNames = append(provisionNames, t.Name)
	}

	name := "new-host"
	id := "id-new-host"

	template := &api.BaseMachineRequest{
		Provision: provisionNames,
	}
	request := new(api.BaseMachineRequest)
	expectReq := new(api.BaseMachineRequest)
	*expectReq = *template
	expectReq.MachineName = name

	provisioner.EXPECT().Name().Return("test-provisioner")
	provisioner.EXPECT().NewRequestInstance().Return(request)
	provisioner.EXPECT().GetProvisionTasks(provisionNames).Return(provision, nil)
	createEvents := make(chan api.CreateInstanceEvent)
	provisioner.EXPECT().CreateInstance(expectReq).Return(createEvents, nil)
	provisioner.EXPECT().GetIPAddress(gomock.Any()).Return("ip", nil)
	provisioner.EXPECT().GetInstanceID(gomock.Any()).Return(id, nil)

	store := mock_storage.NewMockMachines(ctrl)
	store.EXPECT().Save(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)

	record := new(storage.MachineRecord)
	record.MachineName = storage.MachineID(name)
	record.InstanceID = id
	record.AppendChange(&api.BaseMachineRequest{
		MachineName: string(record.MachineName),
		Provision:   provisionNames,
	})
	notFound := &Error{Code: ErrNotFound}
	store.EXPECT().GetRecord(storage.MachineID(record.MachineName)).Times(1).Return(record, notFound)
	store.EXPECT().GetRecord(storage.MachineID(record.MachineName)).AnyTimes().Return(record, nil)
	machines := NewMachines(store)
	events, err := machines.CreateMachine(
		provisioner,
		cred,
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

	provisioner := mock_api.NewMockProvisioner(ctrl)
	cred := mock_api.NewMockCredential(ctrl)

	teardown := []api.Task{
		TaskDestroyInstance,
		TaskSSHKeyRemove,
	}
	teardownNames := []api.TaskName{}
	for _, t := range teardown {
		teardownNames = append(teardownNames, t.Name)
	}

	// Data from previous provisioning run
	record := new(storage.MachineRecord)
	record.MachineName = "test-host"
	record.InstanceID = "id-test-host"
	record.AppendChange(&api.BaseMachineRequest{
		MachineName: string(record.MachineName),
		Teardown:    teardownNames,
	})

	provisioner.EXPECT().GetTeardownTasks(teardownNames).Return(teardown, nil)
	destroyEvents := make(chan api.DestroyInstanceEvent)
	provisioner.EXPECT().DestroyInstance(record.InstanceID).Return(destroyEvents, nil)

	store := mock_storage.NewMockMachines(ctrl)
	store.EXPECT().Save(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	machines := NewMachines(store)
	events, err := machines.DeleteMachine(provisioner, cred, *record)
	require.NoError(t, err)
	require.NotNil(t, events)
	close(destroyEvents) // Simulates the async destroy completes
	checkTasksAreRun(t, events, teardown)
}

func TestRunTasksNoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_api.NewMockProvisioner(ctrl)
	cred := mock_api.NewMockCredential(ctrl)
	request := &api.BaseMachineRequest{}
	record := new(storage.MachineRecord)

	tasks := []api.Task{
		makeTask("step1"),
		makeTask("step2"),
		makeTask("step3"),
		makeTask("step4"),
	}

	events, err := runTasks(
		provisioner,
		tasks,
		record,
		cred,
		request,
		func(r storage.MachineRecord, q api.MachineRequest) error {
			return nil
		},
		func(r *storage.MachineRecord, s api.MachineRequest) {
			return
		})

	require.NoError(t, err)
	require.NotNil(t, events)

	checkTasksAreRun(t, events, tasks)
}

func TestRunTasksErrorAbort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_api.NewMockProvisioner(ctrl)
	cred := mock_api.NewMockCredential(ctrl)
	request := &api.BaseMachineRequest{}
	record := new(storage.MachineRecord)

	tasks := []api.Task{
		makeTask("step1"),
		makeTask("step2"),
		makeErrorTask("step3"),
		makeTask("step4"),
	}

	events, err := runTasks(
		provisioner,
		tasks,
		record,
		cred,
		request,
		func(r storage.MachineRecord, q api.MachineRequest) error {
			return nil
		},
		func(r *storage.MachineRecord, s api.MachineRequest) {
			return
		})

	require.NoError(t, err)
	require.NotNil(t, events)

	checkTasksAreRun(t, events, tasks[0:3])
}
