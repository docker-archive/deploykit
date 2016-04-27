package kv

import (
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/boltdb"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
)

func TestCrud(t *testing.T) {
	dbFile, err := ioutil.TempFile("", "kv_test_boltdb")
	require.Nil(t, err)
	defer os.Remove(dbFile.Name())

	boltdb.Register()
	kv, err := libkv.NewStore(
		store.BOLTDB,
		[]string{dbFile.Name()},
		&store.Config{Bucket: "boltDBTest"})
	require.Nil(t, err)

	storage := NewStore(kv)
	defer storage.Close()

	key1 := "db-16"

	contents, err := storage.ReadAll()
	require.Nil(t, err)
	require.Equal(t, map[string][]byte{}, contents)

	require.NotNil(t, storage.Delete(key1))

	data, err := storage.Read(key1)
	require.NotNil(t, err) // An error is returned for an absent key.
	require.Nil(t, data)

	machineData := []byte("machine information")

	require.Nil(t, storage.Write(key1, machineData))

	contents, err = storage.ReadAll()
	require.Nil(t, err)
	require.Equal(t, map[string][]byte{key1: machineData}, contents)

	data, err = storage.Read(key1)
	require.Nil(t, err)
	require.Equal(t, machineData, data)

	updatedMachineData := []byte("different information")

	require.Nil(t, storage.Write(key1, updatedMachineData))

	data, err = storage.Read(key1)
	require.Nil(t, err)
	require.Equal(t, updatedMachineData, data)

	contents, err = storage.ReadAll()
	require.Nil(t, err)
	require.Equal(t, map[string][]byte{key1: updatedMachineData}, contents)

	key2 := "db-17"
	require.Nil(t, storage.Write(key2, machineData))

	contents, err = storage.ReadAll()
	require.Nil(t, err)
	require.Equal(t, map[string][]byte{key1: updatedMachineData, key2: machineData}, contents)

	require.Nil(t, storage.Delete(key1))

	data, err = storage.Read(key1)
	require.NotNil(t, err)
	require.Nil(t, data)

	contents, err = storage.ReadAll()
	require.Nil(t, err)
	require.Equal(t, map[string][]byte{key2: machineData}, contents)

	require.Nil(t, storage.Delete(key2))

	contents, err = storage.ReadAll()
	require.Nil(t, err)
	require.Equal(t, map[string][]byte{}, contents)
}
