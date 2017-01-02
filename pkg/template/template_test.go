package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunTemplateWithJMESPath(t *testing.T) {

	// Example from http://jmespath.org/
	str := `{{ q "locations[?state == 'WA'].name | sort(@) | {WashingtonCities: join(', ', @)}" . | jsonEncode}}`

	tpl, err := NewTemplate("str://"+str, Options{})
	require.NoError(t, err)

	view, err := tpl.Render(map[string]interface{}{
		"locations": []map[string]interface{}{
			{"name": "Seattle", "state": "WA"},
			{"name": "New York", "state": "NY"},
			{"name": "Bellevue", "state": "WA"},
			{"name": "Olympia", "state": "WA"},
		},
	})

	require.NoError(t, err)
	expected := `{
  "WashingtonCities": "Bellevue, Olympia, Seattle"
}`
	require.Equal(t, expected, view)
}

func TestVarAndExport(t *testing.T) {
	str := `{{ q "locations[?state == 'WA'].name | sort(@) | {WashingtonCities: join(', ', @)}" . | export "washington-cities"}}

{{/* The query above is exported and referenced somewhere else */}}
{
  "test" : "hello",
  "val"  : true,
  "result" : {{var "washington-cities" "A json with washington cities" | jsonEncode}}
}
`

	tpl, err := NewTemplate("str://"+str, Options{})
	require.NoError(t, err)

	view, err := tpl.Render(map[string]interface{}{
		"locations": []map[string]interface{}{
			{"name": "Seattle", "state": "WA"},
			{"name": "New York", "state": "NY"},
			{"name": "Bellevue", "state": "WA"},
			{"name": "Olympia", "state": "WA"},
		},
	})

	require.NoError(t, err)

	// Note the extra newlines because of comments, etc.
	expected := `


{
  "test" : "hello",
  "val"  : true,
  "result" : {
  "WashingtonCities": "Bellevue, Olympia, Seattle"
}
}
`
	require.Equal(t, expected, view)

}
