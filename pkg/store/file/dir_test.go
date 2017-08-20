package file

import (
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestUsage(t *testing.T) {

	dir := os.TempDir()
	typeName := fmt.Sprintf("dirtest-%v", rand.Int63())

	store := NewStore(typeName, dir, true).Init()

	exists, err := store.Exists("hello")
	require.Error(t, err)
	require.False(t, exists)

	err = store.Write("hello", "world")
	require.NoError(t, err)

	exists, err = store.Exists("hello")
	require.NoError(t, err)
	require.True(t, exists)

	read := []string{}
	err = store.All(nil, nil,
		func(buff []byte) (interface{}, error) {
			v := ""
			err := types.AnyYAMLMust(buff).Decode(&v)
			return v, err
		},
		func(v interface{}) (bool, error) {
			read = append(read, v.(string))
			return true, nil
		})

	require.NoError(t, err)
	require.EqualValues(t, []string{"world"}, read)

	object, err := store.Read("hello", func(buff []byte) (interface{}, error) {
		v := ""
		err := types.AnyYAMLMust(buff).Decode(&v)
		return v, err
	})
	require.NoError(t, err)
	require.EqualValues(t, "world", object)
}
