package enrollment

import (
	"fmt"
	"strings"
	"testing"

	"github.com/docker/infrakit/pkg/controller/enrollment/types"
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

	diff := instance.Difference(a, keyFunc, b, keyFunc)
	require.Equal(t, instance.Descriptions{
		{ID: instance.ID("1")},
		{ID: instance.ID("4")},
	}, diff)

	diff2 := instance.Difference(b, keyFunc, a, keyFunc)
	require.Equal(t, instance.Descriptions{
		{ID: instance.ID("6")},
	}, diff2)

	add, remove := Delta(
		instance.Descriptions(a), keyFunc, types.SourceParseErrorEnableDestroy,
		instance.Descriptions(b), keyFunc, types.EnrolledParseErrorEnableProvision,
	)
	require.Equal(t, instance.Descriptions{a[0], a[3]}, add)
	require.Equal(t, instance.Descriptions{b[3]}, remove)
}

func TestDifferenceError(t *testing.T) {
	common := instance.Description{ID: instance.ID("common")}

	a1 := instance.Description{ID: instance.ID("a1")}
	a2 := instance.Description{ID: instance.ID("a2")}
	aError := instance.Description{ID: instance.ID("error-a")}
	aValid := instance.Descriptions{common, a1, a2}
	aParseError := instance.Descriptions{common, a1, a2, aError}

	b1 := instance.Description{ID: instance.ID("b1")}
	b2 := instance.Description{ID: instance.ID("b2")}
	bError := instance.Description{ID: instance.ID("error-b")}
	bValid := instance.Descriptions{common, b1, b2}
	bParseError := instance.Descriptions{common, b1, b2, bError}

	keyFunc := func(i instance.Description) (string, error) {
		if strings.HasPrefix(string(i.ID), "error") {
			return "", fmt.Errorf("ID-error")
		}
		return string(i.ID), nil
	}

	// No errors, should be the same with any operation (even invalid ones)
	diff := instance.Difference(aValid, keyFunc, bValid, keyFunc)
	require.Equal(t, instance.Descriptions{a1, a2}, diff)
	for _, op1 := range []string{types.SourceParseErrorEnableDestroy, types.SourceParseErrorDisableDestroy, "bogus"} {
		for _, op2 := range []string{types.EnrolledParseErrorEnableProvision, types.EnrolledParseErrorDisableProvision, "bogus"} {
			add, remove := Delta(
				instance.Descriptions(aValid), keyFunc, op1,
				instance.Descriptions(bValid), keyFunc, op2,
			)
			require.Equal(t, instance.Descriptions{a1, a2}, add)
			require.Equal(t, instance.Descriptions{b1, b2}, remove)
		}
	}

	// Source fails
	diff = instance.Difference(aParseError, keyFunc, bValid, keyFunc)
	require.Equal(t, instance.Descriptions{a1, a2}, diff)
	// Source failed to parse, disable destroy
	add, remove := Delta(
		instance.Descriptions(aParseError), keyFunc, types.SourceParseErrorDisableDestroy,
		instance.Descriptions(bValid), keyFunc, "",
	)
	require.Equal(t, instance.Descriptions{a1, a2}, add)
	require.Equal(t, instance.Descriptions{}, remove)
	// Source failed to parse, enable the destroy
	add, remove = Delta(
		instance.Descriptions(aParseError), keyFunc, types.SourceParseErrorEnableDestroy,
		instance.Descriptions(bValid), keyFunc, "",
	)
	require.Equal(t, instance.Descriptions{a1, a2}, add)
	require.Equal(t, instance.Descriptions{b1, b2}, remove)

	// Enrollment failed
	diff = instance.Difference(aValid, keyFunc, bParseError, keyFunc)
	require.Equal(t, instance.Descriptions{a1, a2}, diff)
	// Enrolled failed to parse, disable the provision
	add, remove = Delta(
		instance.Descriptions(aValid), keyFunc, "",
		instance.Descriptions(bParseError), keyFunc, types.EnrolledParseErrorDisableProvision,
	)
	require.Equal(t, instance.Descriptions{}, add)
	require.Equal(t, instance.Descriptions{b1, b2}, remove)
	// Enrolled failed to parse, enable the provision
	add, remove = Delta(
		instance.Descriptions(aValid), keyFunc, "",
		instance.Descriptions(bParseError), keyFunc, types.EnrolledParseErrorEnableProvision,
	)
	require.Equal(t, instance.Descriptions{a1, a2}, add)
	require.Equal(t, instance.Descriptions{b1, b2}, remove)

	// Both fail
	diff = instance.Difference(aParseError, keyFunc, bParseError, keyFunc)
	require.Equal(t, instance.Descriptions{a1, a2}, diff)
	// Disable provision and destroy, nothing should be changed
	add, remove = Delta(
		instance.Descriptions(aParseError), keyFunc, types.SourceParseErrorDisableDestroy,
		instance.Descriptions(bParseError), keyFunc, types.EnrolledParseErrorDisableProvision,
	)
	require.Equal(t, instance.Descriptions{}, add)
	require.Equal(t, instance.Descriptions{}, remove)
	// Enable destroy only
	add, remove = Delta(
		instance.Descriptions(aParseError), keyFunc, types.SourceParseErrorEnableDestroy,
		instance.Descriptions(bParseError), keyFunc, types.EnrolledParseErrorDisableProvision,
	)
	require.Equal(t, instance.Descriptions{}, add)
	require.Equal(t, instance.Descriptions{b1, b2}, remove)
	// Enable provision only
	add, remove = Delta(
		instance.Descriptions(aParseError), keyFunc, types.SourceParseErrorDisableDestroy,
		instance.Descriptions(bParseError), keyFunc, types.EnrolledParseErrorEnableProvision,
	)
	require.Equal(t, instance.Descriptions{a1, a2}, add)
	require.Equal(t, instance.Descriptions{}, remove)
	// Enable provision and destroy
	add, remove = Delta(
		instance.Descriptions(aParseError), keyFunc, types.SourceParseErrorEnableDestroy,
		instance.Descriptions(bParseError), keyFunc, types.EnrolledParseErrorEnableProvision,
	)
	require.Equal(t, instance.Descriptions{a1, a2}, add)
	require.Equal(t, instance.Descriptions{b1, b2}, remove)
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

	diff := instance.Difference(a, aKeyFunc, b, bKeyFunc)
	require.Equal(t, instance.Descriptions{a[1], a[4]}, diff)

	diff2 := instance.Difference(b, bKeyFunc, a, aKeyFunc)
	require.Equal(t, instance.Descriptions{b[3], b[4]}, diff2)

	add, remove := Delta(
		instance.Descriptions(a), aKeyFunc, types.SourceParseErrorDisableDestroy,
		instance.Descriptions(b), bKeyFunc, types.EnrolledParseErrorDisableProvision,
	)
	require.Equal(t, instance.Descriptions{a[1], a[4]}, add)
	require.Equal(t, instance.Descriptions{b[3], b[4]}, remove)
}
