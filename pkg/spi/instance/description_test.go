package instance

import (
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func testView(d Description, v string) string {
	vv, err := d.View(v)
	if err != nil {
		panic(err)
	}
	return vv
}

func TestView(t *testing.T) {

	require.Equal(t, "id", testView(Description{
		ID: ID("id"),
	}, `{{.ID}}`))

	require.Equal(t, "id", testView(Description{
		LogicalID: LogicalIDFromString("id"),
	}, `{{.LogicalID}}`))

	require.Equal(t, "foo", testView(Description{
		Tags: map[string]string{
			"bar": "foo",
		},
	}, `{{.Tags.bar}}`))

	require.Equal(t, "foo", testView(Description{
		Tags: map[string]string{
			"bar-bar": "foo",
		},
	}, `{{ index .Tags "bar-bar" }}`))

	// Note that we can index into a types.Any
	require.Equal(t, "foobar", testView(Description{
		Tags: map[string]string{
			"bar-bar": "foo",
		},
		Properties: types.AnyValueMust(map[string]interface{}{"Bar": "bar"}),
	}, `{{ index .Tags "bar-bar" }}{{ .Properties.Bar }}`))
}

func TestDifference(t *testing.T) {

	a := Descriptions{
		{ID: ID("1")},
		{ID: ID("2")},
		{ID: ID("3")},
		{ID: ID("4")},
		{ID: ID("5")},
	}

	b := Descriptions{
		{ID: ID("2")},
		{ID: ID("3")},
		{ID: ID("5")},
		{ID: ID("6")},
	}

	keyFunc := func(i Description) (string, error) {
		return string(i.ID), nil
	}

	diff := Difference(a, keyFunc, b, keyFunc)
	require.Equal(t, Descriptions{
		{ID: ID("1")},
		{ID: ID("4")},
	}, diff)

	diff2 := Difference(b, keyFunc, a, keyFunc)
	require.Equal(t, Descriptions{
		{ID: ID("6")},
	}, diff2)

}

func TestDifference2(t *testing.T) {

	a := Descriptions{
		{ID: ID("0x"), LogicalID: LogicalIDFromString("0")},
		{ID: ID("1x"), LogicalID: LogicalIDFromString("1")},
		{ID: ID("2x"), LogicalID: LogicalIDFromString("2")},
		{ID: ID("3x"), LogicalID: LogicalIDFromString("3")},
		{ID: ID("4x"), LogicalID: LogicalIDFromString("4")},
	}

	b := Descriptions{
		{ID: ID("0")},
		{ID: ID("2")},
		{ID: ID("3")},
		{ID: ID("5")},
		{ID: ID("6")},
	}

	aKeyFunc := func(i Description) (string, error) {
		return string(*i.LogicalID), nil
	}
	bKeyFunc := func(i Description) (string, error) {
		return string(i.ID), nil
	}

	diff := Difference(a, aKeyFunc, b, bKeyFunc)
	require.Equal(t, Descriptions{a[1], a[4]}, diff)

	diff2 := Difference(b, bKeyFunc, a, aKeyFunc)
	require.Equal(t, Descriptions{b[3], b[4]}, diff2)

	aIndex, err := a.Index(aKeyFunc)
	require.NoError(t, err)
	bIndex, err := b.Index(bKeyFunc)
	require.NoError(t, err)
	require.Equal(t, Descriptions{a[1], a[4]}, aIndex.Select(aIndex.Keys.Difference(bIndex.Keys)))
	require.Equal(t, Descriptions{b[3], b[4]}, bIndex.Select(bIndex.Keys.Difference(aIndex.Keys)))
}

func TestCompare(t *testing.T) {

	a := Description{ID: ID("1")}
	b := Description{ID: ID("2")}

	require.Equal(t, 0, a.Compare(a))
	require.Equal(t, -1, a.Compare(b))
	require.Equal(t, 1, b.Compare(a))
}
