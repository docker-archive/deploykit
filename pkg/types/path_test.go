package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPath(t *testing.T) {

	p1 := PathFromString("a/b/c")
	p2 := PathFromString("b/c")

	require.Equal(t, "a", *p1.Index(0))
	require.Equal(t, p2, p1.Shift(1))

	require.Equal(t, "c", p1.Base())
	require.Equal(t, PathFromString("a/b"), PathFromString("a/b/c").Dir())

	require.Equal(t, "a", PathFromString("a").Base())
	require.Equal(t, PathFromString("."), PathFromString("a").Dir())
	require.True(t, PathFromString(".").Dot())
	require.False(t, PathFromString("./a").Dot())

	require.Equal(t, PathFromString("a/b/c/d/"), PathFromString("a/b/c/d/.").Clean())
	require.Equal(t, PathFromString("a/b/c"), PathFromString("a/b/c/d/..").Clean())
	require.Equal(t, PathFromString("a/b/c/"), PathFromString("a/b/c/d/../").Clean())
	require.Equal(t, PathFromString("a/b/c/"), PathFromString("./a/b/c/d/../").Clean())
	require.Equal(t, PathFromString("."), PathFromString("a/..").Clean())

	require.Equal(t, PathFromString("a/b/c/d/"), PathFromString("a/b/c/d/").Clean())
	require.Equal(t, PathFromString("a/b/c/d"), PathFromString("a/b/c/").JoinString("d"))
	require.Equal(t, PathFromString("a/b/c/d/x/y"), PathFromString("a/b/c/").Join(PathFromString("d/x/y")))

	require.Equal(t, PathFromString("c/d/e/f"), PathFromString("a/b/c/d/e/f").Rel(PathFromString("a/b/")))
	require.Equal(t, PathFromString("a"), PathFromString("a").Rel(PathFromString(".")))

	require.True(t, PathFromString("a/b").Less(PathFromString("b")))
	require.True(t, PathFromString("a/b/c/d").Less(PathFromString("b/c/d")))
	require.True(t, PathFromString("a/b/c/d").Less(PathFromString("a/b/c/d/e")))
	require.False(t, PathFromString("x/a/b/c/d").Less(PathFromString("a/b/c/d/e")))

	require.True(t, PathFromString("a/b").Equal(PathFromString("a/b")))
	require.False(t, PathFromString("a/b/c/d").Equal(PathFromString("b/c/d")))

	list := PathFromStrings("a/b/c/d", "x/y/z", "a/b/e/f", "k/z/y/1")
	Sort(list)

	list2 := PathFromStrings("a/b/c/d", "a/b/e/f", "k/z/y/1", "x/y/z")
	require.Equal(t, list, list2)
}
