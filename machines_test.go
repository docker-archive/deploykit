package libmachete

import (
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
//go:generate mockgen -package api -destination mock/provisioners/api/machine_request.go github.com/docker/libmachete/provisioners/api MachineRequest

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
		Id     storage.MachineID
		Record storage.MachineRecord
	}{
		{Id: storage.MachineID("host2"), Record: machineRecord("host2")},
		{Id: storage.MachineID("host1"), Record: machineRecord("host1")},
		{Id: storage.MachineID("host3"), Record: machineRecord("host3")},
	}

	store := mock_storage.NewMockMachines(ctrl)

	ids := []storage.MachineID{}
	for _, m := range data {
		ids = append(ids, m.Id)
	}
	t.Log("ids=", ids)

	store.EXPECT().List().Return(ids, nil)

	for _, m := range data {
		result := m.Record
		store.EXPECT().GetRecord(gomock.Eq(m.Id)).Return(&result, nil)
	}

	machines := NewMachines(store)

	summaries, err := machines.List()
	require.Nil(t, err)

	names := []string{}
	for _, s := range summaries {
		names = append(names, string(s.MachineName))
	}
	require.Equal(t, []string{"host1", "host2", "host3"}, names)

	// Another -- test listing of ids only
	store.EXPECT().List().Return(ids, nil)

	l, err := machines.ListIds()
	require.Nil(t, err)
	require.Equal(t, names, l)
}

func TestRunTasksNoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_api.NewMockProvisioner(ctrl)
	cred := mock_api.NewMockCredential(ctrl)
	request := mock_api.NewMockMachineRequest(ctrl)
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
		nil,
		cred,
		request,
		func(r storage.MachineRecord, q api.MachineRequest) error {
			return nil
		},
		func(r *storage.MachineRecord, s api.MachineRequest) {
			return
		})

	require.Nil(t, err)
	require.NotNil(t, events)

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
	require.Equal(t, []string{"step1", "step2", "step3", "step4"}, executed)
}

func TestRunTasksErrorAbort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_api.NewMockProvisioner(ctrl)
	cred := mock_api.NewMockCredential(ctrl)
	request := mock_api.NewMockMachineRequest(ctrl)
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
		nil,
		cred,
		request,
		func(r storage.MachineRecord, q api.MachineRequest) error {
			return nil
		},
		func(r *storage.MachineRecord, s api.MachineRequest) {
			return
		})

	require.Nil(t, err)
	require.NotNil(t, events)

	seen := []interface{}{}

	// extract the name of the events
	executed := []string{}

	for e := range events {
		seen = append(seen, e)

		if evt, ok := e.(storage.Event); ok {
			executed = append(executed, evt.Name)
		}
	}

	require.Equal(t, len(tasks)-1, len(seen))
	require.Equal(t, []string{"step1", "step2", "step3"}, executed)
}
