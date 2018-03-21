package playbook

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	. "github.com/docker/infrakit/pkg/testing"
)

func TestParse(t *testing.T) {

	// first the high level index file
	buff := `
command1 : file1
command2 : file2
group    : file3
`

	m, err := dir(SourceURL("str://"+buff), Options{})
	require.NoError(t, err)
	require.Equal(t, Modules{
		Op("command1"): SourceURL("file1"),
		Op("command2"): SourceURL("file2"),
		Op("group"):    SourceURL("file3"),
	}, m)
	T(100).Info("module=", m)
}

func TestLoadModules(t *testing.T) {
	pwd, err := os.Getwd()
	require.NoError(t, err)

	top := filepath.Join(pwd, "testdata/index.ikm")

	options := Options{}

	root := SourceURL("file://" + top)
	m, err := dir(root, options)
	require.NoError(t, err)

	T(100).Infoln(m)

	commands, err := list(context.Background(), scope.Nil, m, os.Stdin, nil, &root, options)
	require.NoError(t, err)
	require.Equal(t, 3, len(commands))
}

func TestLoadAll(t *testing.T) {
	pwd, err := os.Getwd()
	require.NoError(t, err)

	root := SourceURL("file://" + filepath.Join(pwd, "testdata/index.ikm"))
	top := Modules{
		Op("testdata"): root,
	}

	modules, err := NewModules(scope.Nil, top, os.Stdin, Options{})
	require.NoError(t, err)

	commands, err := modules.List()
	require.NoError(t, err)
	require.Equal(t, 1, len(commands))
	require.Equal(t, 3, len(commands[0].Commands()))
	require.Equal(t, "testdata", commands[0].Name())

	s := []string{}
	for _, c := range commands[0].Commands() {
		s = append(s, c.Name())
	}
	sort.Strings(s)
	require.Equal(t, []string{"mod1", "mod2", "mod3"}, s)

	cmd := (*cobra.Command)(nil)
	for _, c := range commands[0].Commands() {
		if c.Name() == "mod1" {
			cmd = c
			break
		}
	}
	require.Equal(t, 2, len(cmd.Commands()))
}
