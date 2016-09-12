package scaler

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

const (
	data = `{
	"a": "a",
	"b": "b",
	"c": {
	  "d": "d",
	  "e": "e"
	}
}`

	reordered = `{
	"a": "a",
	"c": {
	  "e": "e",
	  "d": "d"
	},
	"b": "b"
}`

	different = `{
	"a": "a",
	"c": {
	  "d": "d"
	}
}`
)

func TestInstanceConfigHash(t *testing.T) {
	require.Equal(t, instanceConfigHash(json.RawMessage(data)), instanceConfigHash(json.RawMessage(data)))
	require.Equal(t, instanceConfigHash(json.RawMessage(data)), instanceConfigHash(json.RawMessage(reordered)))
	require.NotEqual(t, instanceConfigHash(json.RawMessage(data)), instanceConfigHash(json.RawMessage(different)))
}
