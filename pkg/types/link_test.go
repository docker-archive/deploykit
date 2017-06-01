package types

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRandomAlphaNumericString(t *testing.T) {
	VerifyRandomAlphaNumericString(t, 15)
	VerifyRandomAlphaNumericString(t, 150)
}

// VerifyRandomAlphaNumericString Verifies the length and characters of the string created from randomAlphaNumericString with the given length
func VerifyRandomAlphaNumericString(t *testing.T, length int) {
	actual := randomAlphaNumericString(length)
	// Verify the length is as expected
	require.Equal(t, length, len(actual), fmt.Sprintf("Unexpected length of string %v: Expected %v but was %v\n", actual, length, len(actual)))

	// Verify the characters are as expected
	regex := "[a-z0-9]"
	validString := regexp.MustCompile(regex)
	require.True(t, validString.MatchString(actual), fmt.Sprintf("Invalid characters found in string: %v. Valid characters are %v", actual, regex))
}

func TestNewLink(t *testing.T) {
	link := NewLink()
	require.NotNil(t, link.Value())
	require.Equal(t,
		16,
		len(link.Value()),
		fmt.Sprintf("Unexpected length of: Expected 16 but was %v\n", len(link.Value())))
	require.Equal(t, "", link.Context())
	// The created date is time.Now(), verify the bounds of this
	require.True(t, link.Created().After(time.Time{}))
	require.False(t, link.Created().After(time.Now()))
}

func TestLabel(t *testing.T) {
	require.Equal(t, NewLink().Label(), LinkLabel)
}

func TestIsValid(t *testing.T) {
	linkNotValid := Link{}
	require.False(t, linkNotValid.Valid())
	linkValid := NewLink()
	require.True(t, linkValid.Valid())
}

func TestNewLinkFromMapValidDate(t *testing.T) {
	label := "label"
	context := "context"
	created := "2017-05-25T19:41:07Z"
	link := NewLinkFromMap(map[string]string{
		LinkLabel:        label,
		LinkContextLabel: context,
		LinkCreatedLabel: created,
	})
	require.True(t, link.Valid())
	require.Equal(t, label, link.Value())
	require.Equal(t, context, link.Context())
	parsedCreated, err := time.Parse(time.RFC3339, created)
	require.NoError(t, err)
	require.Equal(t, parsedCreated, link.Created())
}

func TestNewLinkFromMapValidDateCaseInsensitve(t *testing.T) {
	label := "label"
	context := "context"
	created := "2017-05-25t19:41:07z"
	link := NewLinkFromMap(map[string]string{
		LinkLabel:        label,
		LinkContextLabel: context,
		LinkCreatedLabel: created,
	})
	require.True(t, link.Valid())
	require.Equal(t, label, link.Value())
	require.Equal(t, context, link.Context())
	parsedCreated, err := time.Parse(time.RFC3339, strings.ToUpper(created))
	require.NoError(t, err)
	require.Equal(t, parsedCreated, link.Created())
}

func TestNewLinkFromMapInvalidDate(t *testing.T) {
	label := "label"
	context := "context"
	created := "not-a-valid-date"
	link := NewLinkFromMap(map[string]string{
		LinkLabel:        label,
		LinkContextLabel: context,
		LinkCreatedLabel: created,
	})
	require.True(t, link.Valid())
	require.Equal(t, label, link.Value())
	require.Equal(t, context, link.Context())
	// Created is still default value
	require.Equal(t, time.Time{}, link.Created())
}

func TestWithContext(t *testing.T) {
	link := NewLink()
	require.Equal(t, "", link.Context())
	context := "context"
	link.WithContext(context)
	require.Equal(t, context, link.Context())
}

func TestKVPairs(t *testing.T) {
	label := "label"
	context := "context"
	created := "2017-05-25T19:41:07Z"
	link := NewLinkFromMap(map[string]string{
		LinkLabel:        label,
		LinkContextLabel: context,
		LinkCreatedLabel: created,
		"foo":            "bar",
	})
	parsedCreated, err := time.Parse(time.RFC3339, created)
	require.NoError(t, err)
	expected := []string{
		fmt.Sprintf("%s=%s", LinkLabel, label),
		fmt.Sprintf("%s=%s", LinkContextLabel, context),
		fmt.Sprintf("%s=%s", LinkCreatedLabel, parsedCreated.Format(time.RFC3339)),
	}
	sort.Strings(expected)
	actual := link.KVPairs()
	sort.Strings(actual)
	require.Equal(t, expected, actual)
}

func TestLinkMap(t *testing.T) {
	label := "label"
	context := "context"
	created := "2017-05-25T19:41:07Z"
	link := NewLinkFromMap(map[string]string{
		LinkLabel:        label,
		LinkContextLabel: context,
		LinkCreatedLabel: created,
		"foo":            "bar",
	})
	// Create an exact copy
	parsedCreated, err := time.Parse(time.RFC3339, created)
	require.NoError(t, err)
	expected := map[string]string{
		LinkLabel:        label,
		LinkContextLabel: context,
		LinkCreatedLabel: parsedCreated.Format(time.RFC3339),
	}
	require.Equal(t, expected, link.Map())
}

func TestInMap(t *testing.T) {
	label := "label"
	context := "context"
	created := "2017-05-25T19:41:07Z"
	link := NewLinkFromMap(map[string]string{
		LinkLabel:        label,
		LinkContextLabel: context,
		LinkCreatedLabel: created,
	})
	// Copy the original and remove the value
	newLink := make(map[string]string)
	link.WriteMap(newLink)
	require.True(t, link.InMap(newLink))
	delete(newLink, LinkLabel)
	require.False(t, link.InMap(newLink))
	// Change the value
	newLink = make(map[string]string)
	link.WriteMap(newLink)
	link.value = "other-label"
	require.False(t, link.InMap(newLink))
	// Remove the context
	newLink = make(map[string]string)
	link.WriteMap(newLink)
	delete(newLink, LinkContextLabel)
	require.False(t, link.InMap(newLink))
	// Change the context
	newLink = make(map[string]string)
	link.WriteMap(newLink)
	newLink[LinkContextLabel] = "different-context"
	require.False(t, link.InMap(newLink))
	// Date does not matter, InMap should still return true
	newLink = make(map[string]string)
	link.WriteMap(newLink)
	delete(newLink, LinkCreatedLabel)
	require.True(t, link.InMap(newLink))
}

func TestEqual(t *testing.T) {
	label := "label"
	context := "context"
	created := "2017-05-25T19:41:07Z"
	link := NewLinkFromMap(map[string]string{
		LinkLabel:        label,
		LinkContextLabel: context,
		LinkCreatedLabel: created,
	})
	// Create an exact copy that should match
	other := NewLinkFromMap(map[string]string{
		LinkLabel:        label,
		LinkContextLabel: context,
		LinkCreatedLabel: created,
	})
	require.True(t, link.Equal(other))
	// Created value does not affect equality
	other.created = time.Time{}
	require.True(t, link.Equal(other))
	// But the value does
	other.value = "foo"
	require.False(t, link.Equal(other))
	other.value = label
	require.True(t, link.Equal(other))
	// And the context does
	other.context = "foo"
	require.False(t, link.Equal(other))
}
