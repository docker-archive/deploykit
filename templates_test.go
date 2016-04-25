package libmachete

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

type fileWriter struct {
	t   *testing.T
	dir string
}

func (w *fileWriter) write(provisioner string, file string, data string) string {
	provisionerPath := path.Join(w.dir, provisioner)
	stat, err := os.Stat(provisionerPath)
	if err == nil {
		if !stat.IsDir() {
			require.Fail(w.t, provisionerPath, "exists but is not a directory")
		}
	} else {
		if os.IsNotExist(err) {
			err = os.Mkdir(provisionerPath, 0744)
			if err != nil {
				require.Fail(w.t, "Failed to mkdir", provisionerPath)
			}
		} else {
			require.Fail(w.t, "Failed to stat", provisionerPath)
		}
	}

	fullPath := path.Join(provisionerPath, file)
	err = ioutil.WriteFile(fullPath, []byte(data), 0644)
	require.Nil(w.t, err)
	return fullPath
}

func writeAndCheckTemplate(
	t *testing.T,
	writer fileWriter,
	templates Templates,
	provisioner string,
	template string,
	templateData string) {

	writer.write(provisioner, template, templateData)
	data, err := templates.Read(provisioner, template)
	require.Nil(t, err)
	require.Equal(t, templateData, string(data))
}

func TestFileTemplatesRead(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "templates_test")
	require.Nil(t, err)
	defer os.RemoveAll(tempDir)

	writer := fileWriter{t: t, dir: tempDir}

	templates, err := FileTemplates(tempDir)
	require.Nil(t, err)

	writeAndCheckTemplate(t, writer, templates, "chungcloud", "dev", "template1")
	writeAndCheckTemplate(t, writer, templates, "chungcloud", "prod", "template2")
	writeAndCheckTemplate(t, writer, templates, "chungpremiumcloud", "prod", "template3")

	template, err := templates.Read("chungcloud", "test")
	require.NotNil(t, err)
	require.Nil(t, template)
}

func TestFileTemplatesBadInputs(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "templates_test")
	require.Nil(t, err)
	defer os.RemoveAll(tempDir)

	templates, err := FileTemplates(tempDir)
	require.Nil(t, err)

	data, err := templates.Read(
		fmt.Sprintf("bad%sprovisioner", string(os.PathSeparator)),
		"template")
	require.NotNil(t, err, "A provisioner name containing a path separator must error")
	require.Nil(t, data)

	data, err = templates.Read(
		"provisioner",
		fmt.Sprintf("bad%stemplate", string(os.PathSeparator)))
	require.NotNil(t, err, "A template name containing a path separator must error")
	require.Nil(t, data)

	templates, err = FileTemplates("a/path/that/does/not/exist/")
	require.NotNil(t, err, "A non-existent template root must error")
	require.Nil(t, templates)

	writer := fileWriter{t: t, dir: tempDir}

	tempFile := writer.write("nothing", "hello", "")
	templates, err = FileTemplates(tempFile)
	require.NotNil(t, err, "A file must be disallowed as a template root")
	require.Nil(t, templates)
}
