package filestores

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/afero"
	"os"
	"path"
)

const (
	dirPermission  = 0700
	filePermission = 0600
)

type sandbox struct {
	fs  afero.Fs
	dir string
}

func newSandbox(dir string) (*sandbox, error) {
	fs := afero.NewOsFs()
	stat, err := fs.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Directory %s does not exist", dir)
		}
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("%s must be a directory", dir)
	}
	return &sandbox{fs: fs, dir: dir}, nil
}

func (f sandbox) List() ([]string, error) {
	files := []string{}
	contents, err := afero.ReadDir(f.fs, f.dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range contents {
		files = append(files, entry.Name())
	}
	return files, nil
}

func (f sandbox) Mkdir(subPath string) error {
	return f.fs.MkdirAll(path.Join(f.dir, subPath), dirPermission)
}

func (f sandbox) MarshalAndSave(fileName string, s interface{}) error {
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

func (f sandbox) ReadAndUnmarshal(fileName string, record interface{}) error {
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

func (f sandbox) Remove(name string) error {
	return f.fs.Remove(path.Join(f.dir, name))
}

func (f sandbox) RemoveAll(name string) error {
	return f.fs.RemoveAll(path.Join(f.dir, name))
}
