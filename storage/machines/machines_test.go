package storage

import (
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/boltdb"
	"github.com/docker/libmachete/storage/kv"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
)

type PlanetRecord struct {
	Rings int
	Shape string
	Moons []string
}

func TestCrud(t *testing.T) {
	dbFile, err := ioutil.TempFile("", "kv_test_boltdb")
	require.Nil(t, err)
	defer os.Remove(dbFile.Name())

	boltdb.Register()
	kvStore, err := libkv.NewStore(
		store.BOLTDB,
		[]string{dbFile.Name()},
		&store.Config{Bucket: "boltDBTest"})
	require.Nil(t, err)

	storage := kv.NewStore(kvStore)
	defer storage.Close()

	inventory := New(storage)

	record := Record{
		Name:         "Mars",
		Created:      123,
		LastModified: 124,
	}
	planet := PlanetRecord{
		Rings: 0,
		Shape: "spherical",
		Moons: []string{"deimos", "phobos"},
	}

	require.Nil(t, inventory.Save(record, planet))

	records, err := inventory.List()
	require.Nil(t, err)
	require.Equal(t, []*Record{&record}, records)

	savedPlanet := new(PlanetRecord)
	savedRecord, err := inventory.GetRecord(Key(record.Name))
	require.Nil(t, err)
	require.Equal(t, &record, savedRecord)
	err = inventory.GetDetails(Key(record.Name), savedPlanet)
	require.Nil(t, err)
	require.Equal(t, &planet, savedPlanet)
}
