package file

import (
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestUsage(t *testing.T) {

	dir := os.TempDir()
	typeName := fmt.Sprintf("dirtest-%v", rand.Int63())

	fstore := NewStore(typeName, dir)

	exists, err := fstore.Exists("hello")
	require.NoError(t, err)
	require.False(t, exists)

	err = fstore.Write("hello", []byte("world"))
	require.NoError(t, err)

	exists, err = fstore.Exists("hello")
	require.NoError(t, err)
	require.True(t, exists)

	read := []string{}
	err = store.Visit(fstore,
		nil, nil,
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

	buff, err := fstore.Read("hello")
	require.NoError(t, err)
	v := ""
	require.NoError(t, types.AnyYAMLMust(buff).Decode(&v))
	require.EqualValues(t, "world", v)
}

func TestFiles(t *testing.T) {

	dir := os.TempDir()
	typeName := fmt.Sprintf("dirtest-%v", rand.Int63())

	fstore := NewStore(typeName, dir)

	for i := 0; i < 10; i++ {

		f := fmt.Sprintf("hello-%d", i)
		exists, err := fstore.Exists(f)
		require.NoError(t, err)
		require.False(t, exists)

		err = fstore.Write(f, []byte("world"))
		require.NoError(t, err)

		exists, err = fstore.Exists(f)
		require.NoError(t, err)
		require.True(t, exists)

		buff, err := fstore.Read(f)
		require.NoError(t, err)
		require.Equal(t, []byte("world"), buff)
		v := ""
		require.NoError(t, types.AnyYAMLMust(buff).Decode(&v))
		require.EqualValues(t, "world", v)

		read := []string{}
		err = store.Visit(fstore,
			nil, nil,
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
	}

}
