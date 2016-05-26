package filestores

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/afero"
	"os"
	"path"
	"sort"
	"strings"
)

const (
	dirPermission  = 0700
	filePermission = 0600
)

// Sandbox is a subdirectory of a file system.
type Sandbox struct {
	fs  afero.Fs
	dir string
}

// NewSandbox creates a sandbox backed by the provided file system.
func NewSandbox(fs afero.Fs, dir string) Sandbox {
	return Sandbox{fs: fs, dir: dir}
}

// NewOsSandbox creates a sandbox backed by the OS file system.
func NewOsSandbox(dir string) Sandbox {
	return NewSandbox(afero.NewOsFs(), dir)
}

// Mkdirs ensures that the sandbox directory exists.  The creator of a sandbox should call this prior to using a
// sandbox.
func (f Sandbox) Mkdirs() error {
	return f.fs.MkdirAll(f.dir, 0750)
}

// Nested creates a sandbox based on a subdirectory of this sandbox.
func (f Sandbox) Nested(subpath string) Sandbox {
	return Sandbox{fs: f.fs, dir: path.Join(f.dir, subpath)}
}

func (f Sandbox) list() ([]string, error) {
	paths := []string{}

	walker := func(filePath string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			paths = append(paths, strings.TrimPrefix(filePath, f.dir+string(os.PathSeparator)))
		}

		return nil
	}

	err := afero.Walk(f.fs, f.dir, walker)
	if err != nil {
		return nil, err
	}

	// Return in predictable order.
	sort.Strings(paths)

	return paths, nil
}

func (f Sandbox) mkdir(subPath string) error {
	return f.fs.MkdirAll(path.Join(f.dir, subPath), dirPermission)
}

func (f Sandbox) marshalAndSave(fileName string, s interface{}) error {
	fullPath := path.Join(f.dir, fileName)

	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("Failed to marshal record %s: %s", s, err)
	}
	err = afero.WriteFile(f.fs, fullPath, data, filePermission)
	if err != nil {
		return fmt.Errorf("Failed to write to %s: %s", fullPath, err)
	}
	return nil
}

func (f Sandbox) readAndUnmarshal(fileName string, record interface{}) error {
	fullPath := path.Join(f.dir, fileName)

	data, err := afero.ReadFile(f.fs, fullPath)
	if err != nil {
		return fmt.Errorf("Failed to read record %s: %s", fullPath, err)
	}

	err = json.Unmarshal(data, &record)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal record %s: %s", fullPath, err)
	}
	return nil
}

func (f Sandbox) remove(name string) error {
	return f.fs.Remove(path.Join(f.dir, name))
}

func (f Sandbox) removeAll(name string) error {
	return f.fs.RemoveAll(path.Join(f.dir, name))
}

func dirAndFile(filePath string) (string, string) {
	dir, file := path.Split(filePath)
	return strings.TrimSuffix(dir, string(os.PathSeparator)), file
}
