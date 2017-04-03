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

const (
	// DefaultCLIExtension is the file extension used to mark a file as an infrakit cli command.
	// Typically templates have the .ikt extension, while CLI commands have .ikc extension
	DefaultCLIExtension = ".ikc"
)

// Setup sets up the necessary environment for running this module -- ie make sure
// the CLI module directories are present, etc.
func Setup() error {
	dir := Dir()
	if dir == "" {
		return fmt.Errorf("Env not set:%s", cli.CliDirEnvVar)
	}
	fs := afero.NewOsFs()
	exists, err := afero.Exists(fs, dir)
	if err != nil {
		return err
	}
	if !exists {
		log.Warn("Creating directory", "dir", dir)
		err = fs.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

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
	log.Debug("Local modules", "dir", dir)

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
	case strings.Contains(fn, DefaultCLIExtension):
		return false
	}
	return true
}

func commandName(s string) string {
	return strings.Replace(s, DefaultCLIExtension, "", -1)
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
			if !entry.IsDir() && skip(entry.Name()) {
				continue entries
			}
		}

		cmd := &cobra.Command{
			Use:   commandName(entry.Name()),
			Short: commandName(entry.Name()),
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
				log.Debug("Running", "command", entry.Name(), "url", url, "args", args)

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
