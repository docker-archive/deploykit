package resource

import (
	"testing"

	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	"github.com/docker/infrakit/pkg/testing/scope"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestCollection(t *testing.T) {

	c, err := newCollection(
		scope.DefaultScope(),
		resource.Options{})
	require.Error(t, err) // buffer size is 0

	c, err = newCollection(
		scope.DefaultScope(),
		resource.Options{
			LostBufferSize:  100,
			FoundBufferSize: 100,
		})
	require.NoError(t, err)
	require.NotNil(t, c)

}

func TestKeyFromPath(t *testing.T) {

	{
		k, err := keyFromPath(types.PathFromString("mystack/resource/networking/net1/Properties/size"))
		require.NoError(t, err)
		require.Equal(t, "mystack", k)
	}
	{
		k, err := keyFromPath(types.PathFromString("./net1/Properties/size"))
		require.NoError(t, err)
		require.Equal(t, "net1", k)
	}

}

func TestParseDepends(t *testing.T) {
	require.False(t, dependsRegex.MatchString("gopher"))
	require.False(t, dependsRegex.MatchString("@depend()"))
	require.True(t, dependsRegex.MatchString("@depend('./bca/xyz/foo')@"))
	require.True(t, dependsRegex.MatchString("@depend('bca/xyz/foo')@"))
	require.True(t, dependsRegex.MatchString("@depend('bca/xyz/foo/field2')@"))
	require.True(t, dependsRegex.MatchString("@depend('bca/xyz/foo/[2]')@"))

	{
		_, match := parseDepends("foo")
		require.False(t, match)
	}
	{
		_, match := parseDepends("foo/bar/baz")
		require.False(t, match)
	}
	{
		p, match := parseDepends("@depend('foo/bar/baz')@")
		require.True(t, match)
		require.Equal(t, `foo/bar/baz`, p.String())
	}

	{
		var v interface{}
		require.NoError(t, types.Decode([]byte(`
field1: bar
field2: 2
field3: "@depend('net1/foo/bar')@"
`), &v))
		require.Equal(t, []types.Path{types.PathFromString(`net1/foo/bar`)}, parse(v, []types.Path{}))
		require.Equal(t, []types.Path{types.PathFromString(`net1/foo/bar`)}, depends(types.AnyValueMust(v)))
	}
	{
		var v interface{}
		require.NoError(t, types.Decode([]byte(`
field1: bar
field2: 2
`), &v))
		require.Equal(t, []types.Path{}, parse(v, []types.Path{}))
		require.Equal(t, []types.Path{}, depends(types.AnyValueMust(v)))
	}
	{
		var v interface{}
		require.NoError(t, types.Decode([]byte(`
field1: bar
field2: 2
field3: "@depend('net1')@"
field4:
  object_field1 : test
  object_field2 : "@depend('net1/foo/bar/2')@"
field5: "@depend('net1/foo/bar/3')@"
`), &v))
		require.Equal(t, types.PathsFromStrings(
			`net1`,
			`net1/foo/bar/2`,
			`net1/foo/bar/3`,
		), types.Paths(depends(types.AnyValueMust(v))))
	}
	{
		var v interface{}
		require.NoError(t, types.Decode([]byte(`
field1: bar
field2: 2
field3: "@depend('net1/foo/bar/1')@"
field4:
  object_field1 : test
  object_field2 : "@depend('net1/foo/bar/2')@"
  object_field3 :
    - element1: "@depend('net1/foo/bar/3/1')@"
    - element2: "@depend('net1/foo/bar/3/2')@"
    - element3: "@depend('net1/foo/bar/3/3')@"
    - element4: "@depend('net1/foo/bar/3/4')@"
field5: "@depend('net1/foo/bar/4')@"
`), &v))

		list1 := types.PathsFromStrings(
			`net1/foo/bar/1`,
			`net1/foo/bar/2`,
			`net1/foo/bar/3/1`,
			`net1/foo/bar/3/2`,
			`net1/foo/bar/3/3`,
			`net1/foo/bar/3/4`,
			`net1/foo/bar/4`,
		)
		list2 := types.Paths(parse(v, nil))
		list1.Sort()
		list2.Sort()
		require.Equal(t, list1, list2)
	}

}

func TestSubstituteDepends(t *testing.T) {
	{
		var v interface{}
		require.NoError(t, types.Decode([]byte(`
field1: bar
field2: 2
field3: "@depend('net1/foo/bar/1')@"
field4:
  object_field1 : test
  object_field2 : "@depend('net1/foo/bar/2')@"
  object_field3 :
    - element1: "@depend('net1/foo/bar/3/1')@"
    - element2: "@depend('net1/foo/bar/3/2')@"
    - element3: "@depend('net1/foo/bar/3/3')@"
    - element4: "@depend('net1/foo/bar/3/4')@"
field5: "@depend('net1/foo/bar/4')@"
`), &v))

		store := map[string]interface{}{
			`net1/foo/bar/1`:   true,
			`net1/foo/bar/2`:   2,
			`net1/foo/bar/3/1`: "3-1",
			`net1/foo/bar/3/2`: int64(32),
			`net1/foo/bar/3/3`: "3-3",
			`net1/foo/bar/3/4`: []string{"3", "4"},
			`net1/foo/bar/4`:   map[string]string{"foo": "bar"},
		}
		fetch := func(p types.Path) (interface{}, error) {
			return store[p.String()], nil
		}

		vv, substitute := dependV(v, fetch)
		require.True(t, substitute)

		require.Equal(t, store[`net1/foo/bar/1`], types.Get(types.PathFromString(`field3`), vv))
		require.Equal(t, store[`net1/foo/bar/2`], types.Get(types.PathFromString(`field4/object_field2`), vv))
		require.Equal(t, store[`net1/foo/bar/3/1`], types.Get(types.PathFromString(`field4/object_field3/[0]/element1`), vv))
		require.Equal(t, store[`net1/foo/bar/3/2`], types.Get(types.PathFromString(`field4/object_field3/[1]/element2`), vv))
		require.Equal(t, store[`net1/foo/bar/3/3`], types.Get(types.PathFromString(`field4/object_field3/[2]/element3`), vv))
		require.Equal(t, store[`net1/foo/bar/3/4`], types.Get(types.PathFromString(`field4/object_field3/[3]/element4`), vv))
		require.Equal(t, store[`net1/foo/bar/4`], types.Get(types.PathFromString(`field5`), vv))

	}

}

func TestProcessWatches(t *testing.T) {

	watch, watching := processWatches(testProperties(t))

	// check the file... count the number of occurrences
	require.Equal(t, 5, len(watch.watchers["az1-net1"]))
	require.Equal(t, 2, len(watch.watchers["az2-net2"]))
	require.Equal(t, 1, len(watching["az1-net2"]))

}
