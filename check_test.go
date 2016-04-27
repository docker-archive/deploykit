package libmachete

import (
	"github.com/stretchr/testify/require"
	"reflect"
	"sort"
	"testing"
)

func TestChecks(t *testing.T) {

	theName := "name"
	theInt := 10

	v := &struct {
		Name     string
		NamePtr  *string
		Int      int
		IntPtr   *int
		ZeroName string
		NilName  *string
		ZeroInt  int
		NilInt   *int
	}{
		Name:    theName,
		NamePtr: &theName,
		Int:     theInt,
		IntPtr:  &theInt,
	}

	// Since we provide values for the Name, NamePtr, Int, and IntPtr fields, none of these
	// should violate the not nil or not zero rules.
	require.False(t, violateNotNil(reflect.ValueOf(v.Name)))
	require.False(t, violateNotNil(reflect.ValueOf(v.NamePtr)))
	require.False(t, violateNotNil(reflect.ValueOf(v.Int)))
	require.False(t, violateNotNil(reflect.ValueOf(v.IntPtr)))

	require.False(t, violateNotZero(reflect.ValueOf(v.Name)))
	require.False(t, violateNotZero(reflect.ValueOf(v.NamePtr)))
	require.False(t, violateNotZero(reflect.ValueOf(v.Int)))
	require.False(t, violateNotZero(reflect.ValueOf(v.IntPtr)))

	// For the fields that are not set.  We check to see if they violate the not nil/ zero checks.
	require.False(t, violateNotNil(reflect.ValueOf(v.ZeroName)))
	require.True(t, violateNotNil(reflect.ValueOf(v.NilName)))
	require.False(t, violateNotNil(reflect.ValueOf(v.ZeroInt)))
	require.True(t, violateNotNil(reflect.ValueOf(v.NilInt)))

	require.True(t, violateNotZero(reflect.ValueOf(v.ZeroName)))
	require.False(t, violateNotZero(reflect.ValueOf(v.NilName)))
	require.True(t, violateNotZero(reflect.ValueOf(v.ZeroInt)))
	require.False(t, violateNotZero(reflect.ValueOf(v.NilInt)))
}

func requireHasFieldNames(t *testing.T, expect []string, err error) {
	sort.Strings(expect)
	actual := err.(*ErrStructFields).Names
	sort.Strings(actual)
	require.Equal(t, expect, actual)
}

type input struct {
	String string `label:"the_string" check:"not_zero"`
	Int    int    `label:"the_int" check:"not_zero"`
	Bool   bool   `label:"the_bool" check:"not_zero"`

	StringPtr *string `label:"string_ptr" check:"not_nil,not_zero"`
	IntPtr    *int    `label:"int_ptr" check:"not_nil"`
	BoolPtr   *bool   `label:"bool_ptr" check:"not_nil"`

	DontCareString string
	DontCareInt    int
	DontCareBool   bool

	DontCareStringPtr *string
	DontCareIntPtr    *int
	DontCareBoolPTr   *bool
}

func TestCheckFields(t *testing.T) {

	theString := "string"
	theInt := 100
	theBool := true

	// Case - when everything required is provided
	target := &input{
		String: theString,
		Int:    theInt,
		Bool:   theBool,

		StringPtr: &theString,
		IntPtr:    &theInt,
		BoolPtr:   &theBool,
	}
	err := CheckFields(target, nil, nil)
	require.Nil(t, err)

	// Case - when required pointer fields are missing
	target = &input{
		String: theString,
		Int:    theInt,
		Bool:   theBool,
	}
	err = CheckFields(target, nil, nil)
	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"BoolPtr", "IntPtr", "StringPtr"}, err)

	// Case -- when required value fields are missing
	target = &input{
		StringPtr: &theString,
		IntPtr:    &theInt,
		BoolPtr:   &theBool,
	}
	err = CheckFields(target, nil, nil)
	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"Bool", "Int", "String"}, err)

	// Case - when required pointer field is provided but is empty string
	emptyString := ""
	zeroInt := 0
	zeroBool := false
	target = &input{
		StringPtr: &emptyString,
		IntPtr:    &zeroInt,  // zero is allowed (no 'not_zero')
		BoolPtr:   &zeroBool, // zero is allowed (no 'not_zero')
	}
	err = CheckFields(target, nil, nil)
	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"Bool", "Int", "String", "StringPtr"}, err)
}

func TestCheckFieldCallbacks(t *testing.T) {

	// Case - when required pointer field is provided but is empty string
	emptyString := ""
	zeroInt := 0
	zeroBool := false
	target := &input{
		StringPtr: &emptyString,
		IntPtr:    &zeroInt,  // zero is allowed (no 'not_zero')
		BoolPtr:   &zeroBool, // zero is allowed (no 'not_zero')
	}

	missingPtr := new(int)
	zeros := new(int)
	zeroLabels := []string{}

	err := CheckFields(target,
		func(v interface{}, n string, g TagGetter) bool {
			*missingPtr++
			return false
		},
		func(v interface{}, n string, g TagGetter) bool {
			*zeros++

			// Get the label of the field
			zeroLabels = append(zeroLabels, g("label"))
			return false
		})

	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"Bool", "Int", "String", "StringPtr"}, err)
	require.Equal(t, 0, *missingPtr) // We provided a pointer but it points to an empty string, so no missing pointers
	require.Equal(t, 4, *zeros)

	// Test / demo on how to access other field tags --> this is useful for reporting missing yaml or json fields.
	sort.Strings(zeroLabels)
	require.Equal(t, []string{"string_ptr", "the_bool", "the_int", "the_string"}, zeroLabels)

	// Case - callback returns immediately on the first error
	*missingPtr = 0
	*zeros = 0
	err = CheckFields(target,
		func(v interface{}, n string, g TagGetter) bool {
			*missingPtr++
			return true
		},
		func(v interface{}, n string, g TagGetter) bool {
			*zeros++
			return true
		})

	require.NotNil(t, err)
	// String is the first declared field.
	require.Equal(t, []string{"String"}, err.(*ErrStructFields).Names)
	require.Equal(t, 0, *missingPtr)
	require.Equal(t, 1, *zeros)

}

type sub struct {
	StringPtr *string `check:"not_nil,not_zero"`
	IntPtr    *int    `check:"not_nil"`
	BoolPtr   *bool   `check:"not_nil"`
}

type nested struct {
	String string `check:"not_zero"`
	Int    int    `check:"not_zero"`
	Bool   bool   `check:"not_zero"`
	Nested sub
}

func TestCheckFieldsNested(t *testing.T) {

	theString := "string"
	theInt := 100
	theBool := true

	// Case - when everything required is provided
	target := &nested{
		String: theString,
		Int:    theInt,
		Bool:   theBool,
		Nested: sub{
			StringPtr: &theString,
			IntPtr:    &theInt,
			BoolPtr:   &theBool,
		},
	}
	err := CheckFields(target, nil, nil)
	require.Nil(t, err)

	// Case - when required pointer fields are missing
	target = &nested{
		String: theString,
		Int:    theInt,
		Bool:   theBool,
	}
	err = CheckFields(target, nil, nil)
	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"BoolPtr", "IntPtr", "StringPtr"}, err)

	// Case -- when required value fields are missing
	target = &nested{
		Nested: sub{
			StringPtr: &theString,
			IntPtr:    &theInt,
			BoolPtr:   &theBool,
		},
	}
	err = CheckFields(target, nil, nil)
	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"Bool", "Int", "String"}, err)

	// Case - when required pointer field is provided but is empty string
	emptyString := ""
	zeroInt := 0
	zeroBool := false
	target = &nested{
		Nested: sub{
			StringPtr: &emptyString,
			IntPtr:    &zeroInt,  // zero is allowed (no 'not_zero')
			BoolPtr:   &zeroBool, // zero is allowed (no 'not_zero')
		},
	}
	err = CheckFields(target, nil, nil)
	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"Bool", "Int", "String", "StringPtr"}, err)
}
