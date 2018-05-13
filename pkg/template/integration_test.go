package template

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/docker/infrakit/pkg/types"
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
   "userData" : {{ include "script.tpl" . | lines | jsonEncode }},
   {{/* Load from an URL */}}
   "sample" : {{ include "https://httpbin.org/get" }},
   {{/* Load from URL and then parse as JSON then select an attribute */}}
   "originIp" : "{{ include "https://httpbin.org/get" | jsonDecode | q "origin" }}"
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

func TestMissingGlobal(t *testing.T) {
	s := `{{ if not (var "/not/exist")}}none{{else}}here{{end}}`
	tt, err := NewTemplate("str://"+s, Options{})
	require.NoError(t, err)
	view, err := tt.Render(nil)
	require.NoError(t, err)
	require.Equal(t, "none", view)
}

func TestSourceAndDef(t *testing.T) {
	r := `{{ var \"foo\" 100 }}`
	s := `{{ source "str://` + r + `" }}foo={{var "foo"}}`
	tt, err := NewTemplate("str://"+s, Options{})
	require.NoError(t, err)
	view, err := tt.Render(nil)
	require.NoError(t, err)
	require.Equal(t, "foo=100", view)
}

func TestSourceAndGlobal(t *testing.T) {
	r := `{{ var \"foo\" 100 }}`
	s := `{{ source "str://` + r + `" }}foo={{var "foo"}}`
	tt, err := NewTemplate("str://"+s, Options{})
	require.NoError(t, err)
	view, err := tt.Render(nil)
	require.NoError(t, err)
	require.Equal(t, "foo=100", view)
}

func TestIncludeAndGlobal(t *testing.T) {
	r := `{{ var \"foo\" 100 }}` // the child template tries to mutate the global
	s := `{{ include "str://` + r + `" }}foo={{var "foo"}}`
	tt, err := NewTemplate("str://"+s, Options{})
	require.NoError(t, err)
	tt.Global("foo", 200) // set the global of the calling / parent template
	view, err := tt.Render(nil)
	require.NoError(t, err)
	require.Equal(t, "foo=200", view) // parent's not affected by child template
}

func TestSourceAndGlobalWithContext(t *testing.T) {
	ctx := map[string]interface{}{
		"a": 1,
		"b": 2,
	}
	r := `{{ var \"foo\" 100 }}{{$void := set . \"a\" 100}}` // sourced mutates the context
	s := `{{ source "str://` + r + `" }}a={{.a}}`
	tt, err := NewTemplate("str://"+s, Options{})
	require.NoError(t, err)
	view, err := tt.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "a=100", view) // the sourced template mutated the calling template's context.
}

func TestIncludeAndGlobalWithContext(t *testing.T) {
	ctx := map[string]interface{}{
		"a": 1,
		"b": 2,
	}
	r := `{{ var \"foo\" 100 }}{{$void := set . \"a\" 100}}` // included tries to mutate the context
	s := `{{ include "str://` + r + `" }}a={{.a}}`
	tt, err := NewTemplate("str://"+s, Options{})
	require.NoError(t, err)
	view, err := tt.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "a=1", view) // the included template cannot mutate the calling template's context.
}

func TestWithFunctions(t *testing.T) {
	ctx := map[string]interface{}{
		"a": 1,
		"b": 2,
	}
	s := `hello={{hello .a }}`
	tt, err := NewTemplate("str://"+s, Options{})
	require.NoError(t, err)
	view, err := tt.WithFunctions(func() []Function {
		return []Function{
			{
				Name: "hello",
				Func: func(n interface{}) interface{} { return n },
			},
		}
	}).Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "hello=1", view)
}

func TestSourceWithHeaders(t *testing.T) {

	h, context := headersAndContext("foo=bar")
	log.Info("result", "context", context, "headers", h)
	require.Equal(t, interface{}(nil), context)
	require.Equal(t, map[string][]string{"foo": {"bar"}}, h)

	h, context = headersAndContext("foo=bar", "bar=baz", 224)
	log.Info("result", "context", context, "headers", h)
	require.Equal(t, 224, context)
	require.Equal(t, map[string][]string{"foo": {"bar"}, "bar": {"baz"}}, h)

	h, context = headersAndContext("foo=bar", "bar=baz")
	log.Info("result", "context", context, "headers", h)
	require.Equal(t, nil, context)
	require.Equal(t, map[string][]string{"foo": {"bar"}, "bar": {"baz"}}, h)

	h, context = headersAndContext("foo")
	log.Info("result", "context", context, "headers", h)
	require.Equal(t, "foo", context)
	require.Equal(t, map[string][]string{}, h)

	h, context = headersAndContext("foo=bar", map[string]string{"hello": "world"})
	log.Info("result", "context", context, "headers", h)
	require.Equal(t, map[string]string{"hello": "world"}, context)
	require.Equal(t, map[string][]string{"foo": {"bar"}}, h)

	// note we don't have to escape -- use the back quote and the string value is valid
	r := "{{ include `https://httpbin.org/headers` `A=B` `Foo=Bar` `Foo=Bar` `X=1` 100 }}"
	s := `{{ $resp := (source "str://` + r + `" | jsonDecode) }}{{ $resp.headers | jsonEncode}}`
	tt, err := NewTemplate("str://"+s, Options{})
	require.NoError(t, err)
	view, err := tt.Render(nil)
	require.NoError(t, err)

	any := types.AnyString(view)
	headers := map[string]interface{}{}
	require.NoError(t, any.Decode(&headers))

	// Looks like httpbin.org's handling of repeating headers has changed:
	//
	// $ curl -i -H "A:B" -H "Foo:Bar" -H "Foo:Bar" https://httpbin.org/headers
	// HTTP/1.1 200 OK
	// Connection: keep-alive
	// Server: meinheld/0.6.1
	// Date: Thu, 11 May 2017 18:50:08 GMT
	// Content-Type: application/json
	// Access-Control-Allow-Origin: *
	// Access-Control-Allow-Credentials: true
	// X-Powered-By: Flask
	// X-Processed-Time: 0.000694990158081
	// Content-Length: 167
	// Via: 1.1 vegur

	// {
	//   "headers": {
	//     "A": "B",
	//     "Accept": "*/*",
	//     "Connection": "close",
	//     "Foo": "Bar,Bar",
	//     "Host": "httpbin.org",
	//     "User-Agent": "curl/7.43.0"
	//   }
	// }

	require.Equal(t, map[string]interface{}{
		"Foo":             "Bar,Bar",
		"Host":            "httpbin.org",
		"User-Agent":      "Go-http-client/1.1",
		"A":               "B",
		"X":               "1",
		"Accept-Encoding": "gzip",
		"Connection":      "close",
	}, headers)
}
