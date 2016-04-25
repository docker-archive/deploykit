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
// The provided template directory will be used as a root.  It is the responsibility of the caller
// to ensure that the template root directory exists.
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

// Reads a template by name, associated with the provided provisioner name.  Neither the provisioner
// name nor the template name may contain a path separator character.
func (f *fileTemplates) Read(provisioner string, template string) ([]byte, error) {
	if strings.Contains(provisioner, string(os.PathSeparator)) {
		return nil, errors.New("Provisioner name must not contain a path separator")
	}
	if strings.Contains(template, string(os.PathSeparator)) {
		return nil, errors.New("Template name must not contain a path separator")
	}

	return ioutil.ReadFile(path.Join(f.dir, provisioner, template))
}
