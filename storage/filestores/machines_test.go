package filestores

import (
	"github.com/docker/libmachete/storage"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"testing"
)

func expectMachines(t *testing.T, store machines, expected []storage.MachineID) {
	list, err := store.List()
	require.NoError(t, err)
	require.Equal(t, expected, list)
}

func TestMachinesCrud(t *testing.T) {
	fs := afero.NewMemMapFs()
	testPath := ".machete/machines"
	require.NoError(t, fs.Mkdir(testPath, 0700))
	store := machines{sandbox: &sandbox{fs: fs, dir: testPath}}

	db16 := storage.MachineID("db-16")

	// Behavior against an empty store.
	expectMachines(t, store, []storage.MachineID{})
	record, err := store.GetRecord(db16)
	require.Nil(t, record)
	require.Error(t, err)
	require.Error(t, store.GetDetails(db16, Details{}))

	db16Record := storage.MachineRecord{
		Name:         db16,
		Provisioner:  "test",
		Created:      storage.Timestamp(123),
		LastModified: storage.Timestamp(124),
	}
	db16Details := Details{Disks: 5, Attributes: []string{"raid", "x64"}}
	require.NoError(t, store.Save(db16Record, db16Details))

	expectMachines(t, store, []storage.MachineID{db16})

	savedRecord, err := store.GetRecord(db16)
	require.NoError(t, err)
	require.Equal(t, db16Record, *savedRecord)

	savedDetails := Details{}
	require.NoError(t, store.GetDetails(db16, &savedDetails))
	require.Equal(t, db16Details, savedDetails)
}

type Details struct {
	Disks      uint
	Attributes []string
}
