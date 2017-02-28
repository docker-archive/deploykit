package template

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTemplateInclusionFromDifferentSources(t *testing.T) {
	prefix := testSetupTemplates(t, testFiles)

	url := filepath.Join(prefix, "plugin/index.tpl")
	tt, err := NewTemplate(url, Options{})
	require.NoError(t, err)

	view, err := tt.Render(nil)
	require.NoError(t, err)

	parsed := map[string]interface{}{}
	err = json.Unmarshal([]byte(view), &parsed)
	require.NoError(t, err)

	// ensure the userData section is a slice from the lines of the included file.
	require.True(t, len(parsed["userData"].([]interface{})) > 0)

	// ensure the originIp is a selected value from some complex query
	require.True(t, parsed["originIp"].(string) != "")

	// ensure the sample is a complex json structure fetched from the http url
	require.True(t, len(parsed["sample"].(map[string]interface{})) > 0)

	found := false
	for _, l := range parsed["userData"].([]interface{}) {
		// check for the included text of common/setup.sh:
		if l.(string) == "echo \"this is common/setup.sh\"" {
			found = true
		}
	}
	require.True(t, found)
}

// sets up the template files on disk, returns the urlPrefix
func testSetupTemplates(t *testing.T, files map[string]string) string {
	lock.Lock()
	defer lock.Unlock()

	if urlPrefix != "" {
		return urlPrefix
	}

	dir, err := ioutil.TempDir("", "infrakit-templates")
	require.NoError(t, err)

	u, err := filepath.Abs(dir)
	require.NoError(t, err)
	urlPrefix = "file://" + u

	for k, v := range files {
		p := filepath.Join(dir, k)
		os.MkdirAll(filepath.Dir(p), 0744)
		err := ioutil.WriteFile(p, []byte(v), 0644)
		require.NoError(t, err)
	}
	return urlPrefix
}

var (
	lock      sync.Mutex
	urlPrefix = ""

	// The keys are the 'filenames' to write the template body as files in the temp directory.
	testFiles = map[string]string{

		"common/setup.sh": `
echo "this is common/setup.sh"
`,
		"plugin/index.tpl": `
{
   "test" : "test1",
   "description" : "simple template to test the various template functions",
   {{/* Load from from ./ using relative path notation. Then split into lines and json encode */}}
   "userData" : {{ include "script.tpl" . | lines | to_json }},
   {{/* Load from an URL */}}
   "sample" : {{ include "https://httpbin.org/get" }},
   {{/* Load from URL and then parse as JSON then select an attribute */}}
   "originIp" : "{{ include "https://httpbin.org/get" | from_json | q "origin" }}"
}`,

		"plugin/script.tpl": `
#!/bin/bash

# initializeManager
set -o errexit
set -o nounset
set -o xtrace

{{ include "../common/setup.sh" }}

EBS_DEVICE=/dev/xvdf

# Determine whether the EBS volume needs to be formatted.
if [ "$(file -sL $EBS_DEVICE)" = "$EBS_DEVICE: data" ]
then
  echo 'Formatting EBS volume device'
  mkfs -t ext4 $EBS_DEVICE
fi

systemctl stop docker
rm -rf /var/lib/docker

mkdir -p /var/lib/docker
echo "$EBS_DEVICE /var/lib/docker ext4 defaults,nofail 0 2" >> /etc/fstab
mount -a
systemctl start docker
`,
	}
)

func TestTemplateContext(t *testing.T) {

	s := `
{{ inc }}

{{ setString "hello" }}

{{ setBool true }}

{{ range loop 10 }}
  {{ inc }}
{{ end }}

The count is {{count}}
The message is {{str}}

{{ dec }}
{{ range loop 5 }}
  {{ dec }}
{{ end }}

The count is {{count}}
The message is {{str}}
`

	tt, err := NewTemplate("str://"+s, Options{})
	require.NoError(t, err)

	context := &context{}

	_, err = tt.Render(context)
	require.NoError(t, err)

	require.Equal(t, 5, context.Count)
	require.True(t, context.Bool)
	require.Equal(t, 23, context.invokes) // note this is private state not accessible in template
}

func TestIndexIndexOf(t *testing.T) {

	{
		tt, err := NewTemplate("str://{{ index . 1 }}", Options{})
		require.NoError(t, err)

		view, err := tt.Render([]string{"a", "b", "c", "d"})
		require.NoError(t, err)
		require.Equal(t, "b", view)
	}
	{
		tt, err := NewTemplate(`str://{{ index_of "c" . }}`, Options{})
		require.NoError(t, err)

		view, err := tt.Render([]string{"a", "b", "c", "d"})
		require.NoError(t, err)
		require.Equal(t, "2", view)
	}
	{
		tt, err := NewTemplate(`str://{{ index . 0 | cat "index-" | nospace }}`, Options{})
		require.NoError(t, err)

		view, err := tt.Render([]string{"a", "b", "c", "d"})
		require.NoError(t, err)
		require.Equal(t, "index-a", view)
	}
}
