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

	key := "db-16"

	err = storage.Delete(key)
	require.NotNil(t, err)

	data, err := storage.Read(key)
	require.NotNil(t, err) // An error is returned for an absent key.
	require.Nil(t, data)

	machineData := []byte("machine information")

	err = storage.Write(key, machineData)
	require.Nil(t, err)

	data, err = storage.Read(key)
	require.Nil(t, err)
	require.Equal(t, machineData, data)

	updatedMachineData := []byte("different information")

	err = storage.Write(key, updatedMachineData)
	require.Nil(t, err)

	data, err = storage.Read(key)
	require.Nil(t, err)
	require.Equal(t, updatedMachineData, data)

	err = storage.Delete(key)
	require.Nil(t, err)

	data, err = storage.Read(key)
	require.NotNil(t, err)
	require.Nil(t, data)
}
