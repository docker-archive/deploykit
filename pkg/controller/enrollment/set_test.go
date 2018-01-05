package enrollment

import (
	"fmt"
	"strings"
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

	diff, err := Difference(a, keyFunc, b, keyFunc)
	require.NoError(t, err)
	require.Equal(t, instance.Descriptions{
		{ID: instance.ID("1")},
		{ID: instance.ID("4")},
	}, diff)

	diff2, err := Difference(b, keyFunc, a, keyFunc)
	require.NoError(t, err)
	require.Equal(t, instance.Descriptions{
		{ID: instance.ID("6")},
	}, diff2)

	add, remove := Delta(instance.Descriptions(a), keyFunc,
		instance.Descriptions(b), keyFunc)
	require.Equal(t, instance.Descriptions{a[0], a[3]}, add)
	require.Equal(t, instance.Descriptions{b[3]}, remove)
}

func TestDifferenceError(t *testing.T) {
	aValid := instance.Descriptions{{ID: instance.ID("1")}}
	aError := instance.Descriptions{{ID: instance.ID("error-a")}}
	bValid := instance.Descriptions{{ID: instance.ID("2")}}
	bError := instance.Descriptions{{ID: instance.ID("error-b")}}

	keyFunc := func(i instance.Description) (string, error) {
		if strings.HasPrefix(string(i.ID), "error") {
			return "", fmt.Errorf("ID-error")
		}
		return string(i.ID), nil
	}

	// Baseline, should pass
	diff, err := Difference(aValid, keyFunc, bValid, keyFunc)
	require.NoError(t, err)
	require.Equal(t, instance.Descriptions{aValid[0]}, diff)
	add, remove := Delta(instance.Descriptions(aValid), keyFunc,
		instance.Descriptions(bValid), keyFunc)
	require.Equal(t, instance.Descriptions{aValid[0]}, add)
	require.Equal(t, instance.Descriptions{bValid[0]}, remove)

	// Source fails, nothing to add/remove
	diff, err = Difference(aError, keyFunc, bValid, keyFunc)
	require.Error(t, err)
	require.Equal(t, instance.Descriptions{}, diff)
	add, remove = Delta(instance.Descriptions(aError), keyFunc,
		instance.Descriptions(bValid), keyFunc)
	require.Equal(t, instance.Descriptions{}, add)
	require.Equal(t, instance.Descriptions{}, remove)

	// Current set fails, we can add but nothing to remove
	diff, err = Difference(aValid, keyFunc, bError, keyFunc)
	require.Error(t, err)
	require.Equal(t, instance.Descriptions{aValid[0]}, diff)
	add, remove = Delta(instance.Descriptions(aValid), keyFunc,
		instance.Descriptions(bError), keyFunc)
	require.Equal(t, instance.Descriptions{aValid[0]}, add)
	require.Equal(t, instance.Descriptions{}, remove)

	// Both fail, nothing to add remove
	diff, err = Difference(aError, keyFunc, bError, keyFunc)
	require.Error(t, err)
	require.Equal(t, instance.Descriptions{}, diff)
	add, remove = Delta(instance.Descriptions(aError), keyFunc,
		instance.Descriptions(bValid), keyFunc)
	require.Equal(t, instance.Descriptions{}, add)
	require.Equal(t, instance.Descriptions{}, remove)
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

	diff, err := Difference(a, aKeyFunc, b, bKeyFunc)
	require.NoError(t, err)
	require.Equal(t, instance.Descriptions{a[1], a[4]}, diff)

	diff2, err := Difference(b, bKeyFunc, a, aKeyFunc)
	require.NoError(t, err)
	require.Equal(t, instance.Descriptions{b[3], b[4]}, diff2)

	add, remove := Delta(instance.Descriptions(a), aKeyFunc,
		instance.Descriptions(b), bKeyFunc)
	require.Equal(t, instance.Descriptions{a[1], a[4]}, add)
	require.Equal(t, instance.Descriptions{b[3], b[4]}, remove)
}
