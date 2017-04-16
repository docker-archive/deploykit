package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/docker/infrakit/pkg/testing"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSpecs(t *testing.T) {

	dir, err := os.Getwd()
	require.NoError(t, err)

	testdata := filepath.Join(dir, "testdata")
	root := "file://" + testdata
	url := filepath.Join(root, "simple.yml")

	source, buff, err := SpecsFromURL(url)
	require.NoError(t, err)
	require.Equal(t, url, source)
	require.True(t, len(buff) > 0)

	T(100).Infoln(string(buff))

	ordered, err := NormalizeSpecs(url, buff)
	require.NoError(t, err)
	require.Equal(t, 2, len(ordered))

	for _, spec := range ordered {

		require.NotNil(t, spec.Template)
		require.True(t, spec.Template.Absolute())
		T(100).Infoln(spec.Template.String(), "root=", root)
		require.True(t, strings.Contains(spec.Template.String(), root))
	}
}
