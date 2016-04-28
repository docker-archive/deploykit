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

	// For non-pointer fields, `required` implies they are not zero values.
	// So for strings it makes sense but for some types like bool it can be strange / artificial.
	String string `yaml:"the_string" check:"required"`
	Int    int    `yaml:"the_int" check:"required"`
	Bool   bool   `yaml:"the_bool" check:"required"`

	// For pointer fields, required implies that they need to be set to not nil.  The *string further more has
	// to be not-empty.
	StringPtr *string `yaml:"string_ptr" check:"required"`
	IntPtr    *int    `yaml:"int_ptr" check:"required"`
	BoolPtr   *bool   `yaml:"bool_ptr" check:"required"`

	DontCareString string
	DontCareInt    int
	DontCareBool   bool

	DontCareStringPtr *string
	DontCareIntPtr    *int
	DontCareBoolPTr   *bool
}

func TestCheckFields(t *testing.T) {

	theString := "string"
	theInt := 10
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
	err := CheckFields(target, nil)
	require.Nil(t, err)

	// Case - when required pointer fields are missing
	target = &input{
		String: theString,
		Int:    theInt,
		Bool:   theBool,
	}
	err = CheckFields(target, nil)
	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"BoolPtr", "IntPtr", "StringPtr"}, err)

	// Case -- when required value fields are missing
	target = &input{
		StringPtr: &theString,
		IntPtr:    &theInt,
		BoolPtr:   &theBool,
	}
	err = CheckFields(target, nil)
	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"Bool", "Int", "String"}, err)

	// Case - when required pointer field is provided but is empty string
	emptyString := ""
	zeroInt := 0
	zeroBool := false
	target = &input{
		StringPtr: &emptyString,
		IntPtr:    &zeroInt,  // acceptable -- it's not nil but can be 0
		BoolPtr:   &zeroBool, //  acceptable -- it's not nil but can be false
	}
	err = CheckFields(target, nil)
	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"Bool", "Int", "String", "StringPtr"}, err)

	// This case here is artificial and doesn't make much sense.  Include here however to make clear the semantics of
	// required tag in the case of non-pointer fields.
	target = &input{
		String:    emptyString, // required means it can't be empty
		Int:       zeroInt,     // required means it can't be a zero value -- which happens to collide with intended 0
		Bool:      zeroBool,    // required means it can't be a zero value -- which happens to collide with intended false
		StringPtr: &emptyString,
		IntPtr:    &zeroInt,  // acceptable -- it's not nil but can be 0
		BoolPtr:   &zeroBool, //  acceptable -- it's not nil but can be false
	}
	err = CheckFields(target, nil)
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
		IntPtr:    &zeroInt,
		BoolPtr:   &zeroBool,
	}

	missing := new(int)
	missingLabels := []string{}

	err := CheckFields(target,
		func(v interface{}, n string, g TagGetter) bool {
			*missing++

			// Get the label of the field
			missingLabels = append(missingLabels, g("yaml"))
			return false
		})

	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"Bool", "Int", "String", "StringPtr"}, err)
	require.Equal(t, 4, *missing)

	// Test / demo on how to access other field tags --> this is useful for reporting missing yaml or json fields.
	sort.Strings(missingLabels)
	require.Equal(t, []string{"string_ptr", "the_bool", "the_int", "the_string"}, missingLabels)

	// Case - callback returns immediately on the first error
	*missing = 0
	err = CheckFields(target,
		func(v interface{}, n string, g TagGetter) bool {
			*missing++
			return true
		})

	require.NotNil(t, err)
	// String is the first declared field.
	require.Equal(t, []string{"String"}, err.(*ErrStructFields).Names)
	require.Equal(t, 1, *missing)

	// Case - using the provided callback to collect missing YAML fields
	missingFields := []string{}
	err = CheckFields(target, CollectMissingYAMLFields(&missingFields))
	require.NotNil(t, err)
	sort.Strings(missingFields)
	require.Equal(t, []string{"string_ptr", "the_bool", "the_int", "the_string"}, missingFields)

	// Case - test the convenience wrapper
	missingFields = FindMissingFields(target)
	sort.Strings(missingFields)
	require.Equal(t, []string{"string_ptr", "the_bool", "the_int", "the_string"}, missingFields)
}

type sub struct {
	StringPtr *string `yaml:"string_ptr" check:"required"`
	IntPtr    *int    `yaml:"int_ptr" check:"required"`
	BoolPtr   *bool   `yaml:"bool_ptr" check:"required"`
}

type nested struct {
	String string `yaml:"the_string" check:"required"`
	Int    int    `yaml:"the_int" check:"required"`
	Bool   bool   `yaml:"the_bool" check:"required"`
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
	err := CheckFields(target, nil)
	require.Nil(t, err)

	// Case - when required pointer fields are missing
	target = &nested{
		String: theString,
		Int:    theInt,
		Bool:   theBool,
	}
	err = CheckFields(target, nil)
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
	err = CheckFields(target, nil)
	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"Bool", "Int", "String"}, err)

	// Case - when required pointer field is provided but is empty string
	emptyString := ""
	zeroInt := 0
	zeroBool := false
	target = &nested{
		Nested: sub{
			StringPtr: &emptyString,
			IntPtr:    &zeroInt,  // zero is allowed since ptr is set
			BoolPtr:   &zeroBool, // zero is allowed since ptr is set
		},
	}
	err = CheckFields(target, nil)
	require.NotNil(t, err)
	requireHasFieldNames(t, []string{"Bool", "Int", "String", "StringPtr"}, err)

	missingFields := FindMissingFields(target)
	sort.Strings(missingFields)
	require.Equal(t, []string{"string_ptr", "the_bool", "the_int", "the_string"}, missingFields)

}
