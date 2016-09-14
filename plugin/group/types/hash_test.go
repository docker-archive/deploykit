package types

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

func TestInstanceHash(t *testing.T) {
	require.Equal(t, instanceHash(json.RawMessage(data)), instanceHash(json.RawMessage(data)))
	require.Equal(t, instanceHash(json.RawMessage(data)), instanceHash(json.RawMessage(reordered)))
	require.NotEqual(t, instanceHash(json.RawMessage(data)), instanceHash(json.RawMessage(different)))
}
