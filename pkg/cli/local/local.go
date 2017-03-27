package local

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/docker/infrakit/pkg/cli"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/local")

// Dir returns the directory to use for modules, which may be customized by the environment.
func Dir() string {
	if modulesDir := os.Getenv(cli.CliDirEnvVar); modulesDir != "" {
		return modulesDir
	}

	home := os.Getenv("HOME")
	if usr, err := user.Current(); err == nil {
		home = usr.HomeDir
	}
	return filepath.Join(home, ".infrakit/cli")
}

type modules struct {
	Dir string
	fs  afero.Fs
}

type missingDir string

func (e missingDir) Error() string {
	return fmt.Sprintf("missing or not a directory:%s", string(e))
}

// IsMissingDir returns if the error is from missing or not a directory
func IsMissingDir(e error) bool {
	_, is := e.(missingDir)
	return is
}

// NewModules returns an implementation of Modules using data found locally on disk
func NewModules(dir string) (cli.Modules, error) {
	log.Debug("local modules", "dir", dir)

	fs := afero.NewOsFs()

	exists, err := afero.Exists(fs, dir)
	if err != nil {
		return nil, err
	}

	isDir, err := afero.IsDir(fs, dir)
	if err != nil {
		return nil, err
	}

	if !exists || !isDir {
		return nil, missingDir(dir)
	}

	return &modules{
		Dir: dir,
		fs:  fs,
	}, nil
}

func skip(fn string) bool {
	// .something or something~ will be skipped
	switch {
	case strings.LastIndex(fn, "~") == len(fn)-1:
		return true
	case strings.Index(fn, ".") == 0:
		return true
	case strings.Contains(fn, ".md"):
		return true
	}
	return false
}

func list(fs afero.Fs, dir string, parent *cobra.Command) ([]*cobra.Command, error) {
	entries, err := afero.ReadDir(fs, dir)
	if err != nil {
		return nil, err
	}

	mods := []*cobra.Command{}
entries:
	for _, entry := range entries {

		switch entry.Name() {

		case ".short":

			if parent == nil {
				continue entries
			}

			short, err := afero.ReadFile(fs, filepath.Join(dir, entry.Name()))
			if err != nil {
				return nil, err
			}
			parent.Short = strings.Trim(string(short), "\n")

			continue entries

		default:
			if skip(entry.Name()) {
				continue entries
			}
		}

		cmd := &cobra.Command{
			Use:   entry.Name(),
			Short: entry.Name(),
		}

		if entry.IsDir() {
			subs, err := list(fs, filepath.Join(dir, entry.Name()), cmd)
			if err != nil {
				return nil, err
			}
			for _, sub := range subs {
				cmd.AddCommand(sub)
			}
		} else {

			url := "file://" + filepath.Join(dir, entry.Name())
			context := &Context{
				cmd:   cmd,
				src:   url,
				input: os.Stdin,
			}

			cmd.RunE = func(c *cobra.Command, args []string) error {
				log.Debug("Running", "command", entry.Name(), "args", args)

				err := context.loadBackend()
				if err != nil {
					return err
				}
				return context.execute()
			}

			err = context.buildFlags()
			if err != nil {
				return nil, err
			}
		}
		mods = append(mods, cmd)
	}
	return mods, nil
}

// List returns a list of commands defined locally
func (m *modules) List() ([]*cobra.Command, error) {
	return list(m.fs, m.Dir, nil)
}
