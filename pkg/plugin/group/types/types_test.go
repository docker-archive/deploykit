package types

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	specA = `{
  "Instance": {
    "Plugin": "a",
    "Properties": {
      "a": "a",
      "b": "b",
      "c": {
        "d": "d",
        "e": "e"
      }
    }
  },
  "Flavor": {
    "Plugin": "f",
    "Properties": {
	"g": "g"
    }
  }
}`

	reordered = `{
  "Instance": {
    "Plugin": "a",
    "Properties": {
      "a": "a",
      "c": {
        "e": "e",
        "d": "d"
      },
      "b": "b"
    }
  },
  "Flavor": {
    "Plugin": "f",
    "Properties": {
	"g": "g"
    }
  }
}`

	different = `{
  "Instance": {
    "Plugin": "a",
    "Properties": {
      "a": "a",
      "c": {
        "d": "d"
      }
    }
  },
  "Flavor": {
    "Plugin": "f",
    "Properties": {
	"g": "g"
    }
  }
}`
)

func TestInstanceHash(t *testing.T) {
	hash := func(config string) string {
		spec := Spec{}
		err := json.Unmarshal([]byte(config), &spec)
		require.NoError(t, err)
		return spec.InstanceHash()
	}

	require.Equal(t, hash(specA), hash(specA))
	require.Equal(t, hash(specA), hash(reordered))
	require.NotEqual(t, hash(specA), hash(different))
	VerifyValidCharsInHash(t, hash(specA))
	VerifyValidCharsInHash(t, hash(reordered))
	VerifyValidCharsInHash(t, hash(different))
}

func VerifyValidCharsInHash(t *testing.T, hash string) {
	regex := "[a-z0-9]"
	validString := regexp.MustCompile(regex)
	require.True(t, validString.MatchString(hash), fmt.Sprintf("Invalid characters found in string: %v. Valid characters are %v", hash, regex))
}
