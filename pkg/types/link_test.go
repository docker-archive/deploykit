package types

import (
	"fmt"
	"regexp"
	"testing"

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
