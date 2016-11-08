package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/infrakit/spi/group"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestConfigAsFileTree(t *testing.T) {

	c := map[string]interface{}{
		"foo": "bar",
		"bar": "baz",
	}

	buff, err := json.MarshalIndent(c, " ", " ")
	require.NoError(t, err)

	raw := json.RawMessage(buff)

	g := GlobalSpec{
		Groups: map[group.ID]PluginSpec{
			group.ID("Managers"): {
				Plugin:     "test",
				Properties: &raw,
			},
			group.ID("Workers"): {
				Plugin:     "test",
				Properties: &raw,
			},
		},
	}

	root := filepath.Join(os.TempDir(), fmt.Sprintf("infrakit-spec-%d", time.Now().Unix()))
	err = os.MkdirAll(root, 0777)
	require.NoError(t, err)

	fs := afero.NewBasePathFs(afero.NewOsFs(), root)
	err = g.WriteFileTree(fs)
	require.NoError(t, err)

	// check for existence
	for _, p := range []string{
		filepath.Join(root, "Groups/Managers/Properties.json"),
		filepath.Join(root, "Groups/Workers/Properties.json"),
	} {

		f, err := os.Open(p)
		require.NoError(t, err)

		m := map[string]interface{}{}
		err = json.NewDecoder(f).Decode(&m)

		require.NoError(t, err)
		require.Equal(t, c, m)
	}

	gg := new(GlobalSpec)
	err = gg.ReadFileTree(fs)
	require.NoError(t, err)
	require.Equal(t, g, *gg)
}
