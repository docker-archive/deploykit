package metadata

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func p(s string) []string {
	return strings.Split(s, "/")
}

func TestPath(t *testing.T) {

	p1 := Path(p("a/b/c"))
	p2 := Path(p("b/c"))

	require.Equal(t, "a", *p1.Index(0))
	require.Equal(t, p2, p1.Shift(1))

	require.Equal(t, "c", p1.Base())
	require.Equal(t, Path(p("a/b")), Path(p("a/b/c")).Dir())

	require.Equal(t, "a", Path(p("a")).Base())
	require.Equal(t, Path(p(".")), Path(p("a")).Dir())

	require.Equal(t, Path(p("a/b/c/d")), Path(p("a/b/c/d/")).Clean())
	require.Equal(t, Path(p("a/b/c/d")), Path(p("a/b/c/d/.")).Clean())
	require.Equal(t, Path(p("a/b/c")), Path(p("a/b/c/d/..")).Clean())
	require.Equal(t, Path(p("a/b/c")), Path(p("a/b/c/d/../")).Clean())
	require.Equal(t, NullPath, Path(p("a/..")).Clean())

	require.Equal(t, Path(p("a/b/c/d")), Path(p("a/b/c/d/")).Clean())
	require.Equal(t, Path(p("a/b/c/d")), Path(p("a/b/c/")).Join("d"))
	require.Equal(t, Path(p("a/b/c/d/x/y")), Path(p("a/b/c/")).Sub(p("d/x/y")))
}
