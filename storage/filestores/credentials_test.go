package filestores

import (
	"github.com/docker/libmachete/storage"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"testing"
)

func expectCredentials(t *testing.T, store credentials, expected []storage.CredentialsID) {
	list, err := store.List()
	require.NoError(t, err)
	require.Equal(t, expected, list)
}

func TestCredentialsCrud(t *testing.T) {
	fs := afero.NewMemMapFs()
	testPath := ".machete/credentials"
	require.NoError(t, fs.Mkdir(testPath, 0700))
	store := credentials{sandbox: Sandbox{fs: fs, dir: testPath}}

	idA := storage.CredentialsID("id_a")

	// Behavior against an empty store.
	expectCredentials(t, store, []storage.CredentialsID{})
	require.Error(t, store.Delete(idA))
	require.Error(t, store.GetCredentials(idA, CredsA{}))

	credsA := CredsA{Identity: "User A", Secret: "secret data"}
	require.NoError(t, store.Save(idA, credsA))

	storedA := CredsA{}
	require.NoError(t, store.GetCredentials(idA, &storedA))
	require.Equal(t, credsA, storedA)

	idB := storage.CredentialsID("id_b")
	credsB := CredsB{Token: 2326, Roles: []string{"users", "donkeys"}}
	require.NoError(t, store.Save(idB, credsB))
	expectCredentials(t, store, []storage.CredentialsID{idA, idB})

	storedB := CredsB{}
	require.NoError(t, store.GetCredentials(idB, &storedB))
	require.Equal(t, credsB, storedB)

	// TODO(wfarner): Add a test for invalid json, and incorrect struct for unmarshalling.

	require.NoError(t, store.Delete(idA))
	expectCredentials(t, store, []storage.CredentialsID{idB})

	require.NoError(t, store.Delete(idB))
	expectCredentials(t, store, []storage.CredentialsID{})
}

type CredsA struct {
	Identity string
	Secret   string
}

type CredsB struct {
	Token int64
	Roles []string
}
