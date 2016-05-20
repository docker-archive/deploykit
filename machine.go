package libmachete

import (
	"errors"
	"fmt"
	"github.com/docker/libmachete/provisioners"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

// Machine interfaces between provisioner-specific machine templates and provisioners to create
// instances.
type machine struct {
	registry       *provisioners.Registry
	templateLoader TemplateLoader
}

// TemplateLoader looks up and reads template data, scoped by provisioner name.
// Previously Templates
type TemplateLoader interface {
	Read(provisioner string, template string) ([]byte, error)
}

type fileTemplateLoader struct {
	dir string
}

// FileTemplateLoader creates a template reader that looks up templates in a file system directory.
// The provided template directory will be used as a root.  It is the responsibility of the caller
// to ensure that the template root directory exists.
func FileTemplateLoader(dir string) (TemplateLoader, error) {
	stat, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	return &fileTemplateLoader{dir: dir}, nil
}

// Reads a template by name, associated with the provided provisioner name.  Neither the provisioner
// name nor the template name may contain a path separator character.
func (f *fileTemplateLoader) Read(provisioner string, template string) ([]byte, error) {
	if strings.Contains(provisioner, string(os.PathSeparator)) {
		return nil, errors.New("Provisioner name must not contain a path separator")
	}
	if strings.Contains(template, string(os.PathSeparator)) {
		return nil, errors.New("Template name must not contain a path separator")
	}

	return ioutil.ReadFile(path.Join(f.dir, provisioner, template))
}
