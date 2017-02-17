package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFunctionHelp(t *testing.T) {

	s := functionSignature("null", nil)
	require.Equal(t, "no-function", s)

	s = functionSignature("string", "string")
	require.Equal(t, "not-a-function", s)

	s = functionSignature("concat", func(a, b string) (string, error) { return "", nil })
	require.Equal(t, "func(string, string) (string, error)", s)

	s = functionUsage("concat", func(a, b, c string) (string, error) { return "", nil })
	require.Equal(t, `{{ concat "string" "string" "string" }}`, s)

	s = functionUsage("somefun", func(a, b, c string, d int) (string, error) { return "", nil })
	require.Equal(t, `{{ somefun "string" "string" "string" int }}`, s)

	s = functionUsage("somefun", func(a, b, c string, d int, e ...bool) (string, error) { return "", nil })
	require.Equal(t, `{{ somefun "string" "string" "string" int [ bool ... ] }}`, s)

	s = functionUsage("myfunc", func(a, b interface{}) (interface{}, error) { return "", nil })
	require.Equal(t, `{{ myfunc any any }}`, s)

	s = functionUsage("myfunc", func(a []string, b map[string]interface{}) (interface{}, error) { return "", nil })
	require.Equal(t, `{{ myfunc []string map[string]interface{} }}`, s)
}
