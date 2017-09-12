package enrollment

import (
	"testing"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {

	a := instance.Descriptions{
		{ID: instance.ID("1")},
		{ID: instance.ID("2")},
		{ID: instance.ID("3")},
		{ID: instance.ID("4")},
		{ID: instance.ID("5")},
	}

	b := instance.Descriptions{
		{ID: instance.ID("2")},
		{ID: instance.ID("3")},
		{ID: instance.ID("5")},
		{ID: instance.ID("6")},
	}

	keyFunc := func(i instance.Description) (string, error) {
		return string(i.ID), nil
	}

	diff := Difference(a, keyFunc, b, keyFunc)
	require.Equal(t, instance.Descriptions{
		{ID: instance.ID("1")},
		{ID: instance.ID("4")},
	}, diff)

	diff2 := Difference(b, keyFunc, a, keyFunc)
	require.Equal(t, instance.Descriptions{
		{ID: instance.ID("6")},
	}, diff2)

	add, remove, _ := Delta(instance.Descriptions(a), keyFunc,
		instance.Descriptions(b), keyFunc)
	require.Equal(t, instance.Descriptions{a[0], a[3]}, add)
	require.Equal(t, instance.Descriptions{b[3]}, remove)
}

func logicalID(s string) *instance.LogicalID {
	id := instance.LogicalID(s)
	return &id
}

func TestSetKeyFuncs(t *testing.T) {

	a := instance.Descriptions{
		{ID: instance.ID("0x"), LogicalID: logicalID("0")},
		{ID: instance.ID("1x"), LogicalID: logicalID("1")},
		{ID: instance.ID("2x"), LogicalID: logicalID("2")},
		{ID: instance.ID("3x"), LogicalID: logicalID("3")},
		{ID: instance.ID("4x"), LogicalID: logicalID("4")},
	}

	b := instance.Descriptions{
		{ID: instance.ID("0")},
		{ID: instance.ID("2")},
		{ID: instance.ID("3")},
		{ID: instance.ID("5")},
		{ID: instance.ID("6")},
	}

	aKeyFunc := func(i instance.Description) (string, error) {
		return string(*i.LogicalID), nil
	}
	bKeyFunc := func(i instance.Description) (string, error) {
		return string(i.ID), nil
	}

	diff := Difference(a, aKeyFunc, b, bKeyFunc)
	require.Equal(t, instance.Descriptions{a[1], a[4]}, diff)

	diff2 := Difference(b, bKeyFunc, a, aKeyFunc)
	require.Equal(t, instance.Descriptions{b[3], b[4]}, diff2)

	add, remove, change := Delta(instance.Descriptions(a), aKeyFunc,
		instance.Descriptions(b), bKeyFunc)
	require.Equal(t, instance.Descriptions{a[1], a[4]}, add)
	require.Equal(t, instance.Descriptions{b[3], b[4]}, remove)
	require.Equal(t, 3, len(change))
}
