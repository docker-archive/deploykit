package libmachete

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

// Templates looks up and reads template data, scoped by provisioner name.
type Templates interface {
	Read(provisioner string, template string) ([]byte, error)
}

type fileTemplates struct {
	dir string
}

// FileTemplates creates a template reader that looks up templates in a file system directory.
func FileTemplates(dir string) (Templates, error) {
	stat, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	return &fileTemplates{dir: dir}, nil
}

func (f *fileTemplates) Read(provisioner string, template string) ([]byte, error) {
	if strings.Contains(provisioner, string(os.PathSeparator)) {
		return nil, errors.New("Provisioner name must not contain a path separator")
	}
	if strings.Contains(template, string(os.PathSeparator)) {
		return nil, errors.New("Template name must not contain a path separator")
	}

	return ioutil.ReadFile(path.Join(f.dir, provisioner, template))
}
