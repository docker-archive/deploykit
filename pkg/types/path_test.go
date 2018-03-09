package types

import (
	"testing"

	. "github.com/docker/infrakit/pkg/testing"
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

	// Behavior of trailing slashes.. in our case we require explicit setting of trailing slash to indicate a 'directory'
	// otherwise, we assume it's node and not all nodes under that branch.  This is necessary to delineate topics.
	require.Equal(t, PathFromString("/a"), PathFromString("/a/b/..").Clean())
	require.Equal(t, PathFromString("a/"), PathFromString("a/").Clean())
	require.NotEqual(t, PathFromString("a/"), PathFromString("a").Clean())
	require.Equal(t, PathFromString("a/b/c/d/"), PathFromString("a/b/c/d/.").Clean())
	require.Equal(t, PathFromString("a/b/c"), PathFromString("a/b/c/d/..").Clean())
	require.Equal(t, PathFromString("a/b/c/"), PathFromString("a/b/c/d/../").Clean())
	require.Equal(t, PathFromString("a/b/c/"), PathFromString("./a/b/c/d/../").Clean())
	require.Equal(t, PathFromString("a/b/c/d/"), PathFromString("./a/b/c//d//").Clean())
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

	require.True(t, PathFromString("a/b").Less(PathFromString("a/b/c")))

	list := PathsFromStrings("a/b/c/d", "x/y/z", "a/b/e/f", "k/z/y/1", "a/b")
	list.Sort()
	list2 := PathsFromStrings("a/b", "a/b/c/d", "a/b/e/f", "k/z/y/1", "x/y/z")
	require.Equal(t, list, list2)
	require.Equal(t, list, PathsFromStrings(
		"a/b",
		"a/b/c/d",
		"a/b/e/f",
		"k/z/y/1",
		"x/y/z",
	))

	require.NotEqual(t, PathFromString("/a/b/c/d/"), PathFromString("a/b/c/d/.").Clean())
	require.Equal(t, PathFromString("/a/x"), PathFromString("/a/b/../x").Clean())
	require.Equal(t, PathFromString("/a/x/y"), PathFromString("/a/b/../x/y").Clean())

	require.Equal(t, "/", PathFromString("/").String())
	require.Equal(t, ".", PathFromString(".").String())
	require.Equal(t, "/", PathFromString("/.").String())
	require.Equal(t, "a/", PathFromString("a/").String())
	require.Equal(t, "a/b/c/", PathFromString("a/").Join(PathFromString("b/c/")).String())
	require.Equal(t, PathFromString("a/").Join(PathFromString("b/c/")), PathFromString("a/b/c/"))

	require.Equal(t, PathFromString("."), PathFromString("").Clean())

	T(100).Infoln(PathFromString("/a"))

	require.Equal(t, 1, len(PathFromString("")))
	require.Equal(t, ".", PathFromString("").String())

	require.Equal(t, PathFrom("a", "b/c", "d"), RFC6901ToPath("a/b~1c/d"))
	require.Equal(t, PathFrom("a", "b/c", "d~f"), RFC6901ToPath("a/b~1c/d~0f"))

	type test struct {
		Path    Path  `json:"path"`
		PathPtr *Path `json:"pathPtr"`
	}

	input := `
{
	"path" : "foo/bar/baz",
	"pathPtr" : "github.com/docker/infrakit/pkg/testing"
}
`

	decoded := test{}

	err := AnyString(input).Decode(&decoded)
	require.NoError(t, err)

	require.Equal(t, PathFromString("foo/bar/baz"), decoded.Path)
	require.Equal(t, PathFromString("github.com/docker/infrakit/pkg/testing"), *decoded.PathPtr)

	any := AnyValueMust(decoded)
	require.Equal(t, `{
"path": "foo/bar/baz",
"pathPtr": "github.com/docker/infrakit/pkg/testing"
}`, any.String())
}
