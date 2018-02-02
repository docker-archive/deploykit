package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type testSpec2 struct {
	Properties *Any `json:"properties,omitempty"`
}

type testSpec struct {
	Properties *Any      `json:"properties,omitempty"`
	Nested     testSpec2 `json:"nested"`
}

type testCantMarshal struct {
	Private func() error
}

func TestMarshalUnmarshalAny(t *testing.T) {

	config := AnyBytes([]byte(`{"name":"config"}`))
	configCopy := AnyString(`{"name":"config"}`)
	require.Equal(t, config.String(), configCopy.String())

	config1, err := AnyValue(map[string]interface{}{"name": "config1"})
	require.NoError(t, err)
	config2, err := AnyValue(map[string]interface{}{"name": "config2"})
	require.NoError(t, err)

	spec := testSpec{
		Properties: config1,
		Nested: testSpec2{
			Properties: config2,
		},
	}

	any, err := AnyValue(spec)
	require.NoError(t, err)

	// now take the encoded buffer and use Any to parse it into a typed struct
	parsedSpec := testSpec{}
	any2 := AnyBytes(any.Bytes())
	err = any2.Decode(&parsedSpec)
	require.NoError(t, err)
	require.Equal(t, any, any2)

	buff1, err := json.MarshalIndent(spec, "", "  ")
	require.NoError(t, err)
	buff2, err := json.MarshalIndent(parsedSpec, "", "  ")
	require.NoError(t, err)
	require.Equal(t, buff1, buff2)

	caughtErr := make(chan interface{}, 1)
	var notHere chan interface{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				caughtErr <- r
			}
		}()
		spec = testSpec{
			Properties: AnyValueMust(testCantMarshal{Private: func() error { return nil }}),
			Nested: testSpec2{
				Properties: AnyValueMust(nil),
			},
		}

		notHere <- 1 // don't expect to come here; here will make this test hang because writing to nil channel blocks

		any3 := AnyValueMust(spec)
		t.Log(any3.String())
	}()

	<-caughtErr // will be stuck fi we didn't get a value

}

func TestMarshalUnmarshalAny2(t *testing.T) {

	config := AnyBytes([]byte(`{"name":"config"}`))
	configCopy := AnyString(`{"name":"config"}`)
	require.Equal(t, config.String(), configCopy.String())

	config1, err := AnyValue(map[string]interface{}{"name": "config1"})
	require.NoError(t, err)
	config2, err := AnyValue(map[string]interface{}{"name": "config2"})
	require.NoError(t, err)

	spec := testSpec{
		Properties: config1,
		Nested: testSpec2{
			Properties: config2,
		},
	}

	any, err := AnyValue(spec)
	require.NoError(t, err)

	_, err = any.MarshalJSON()
	require.NoError(t, err)

	// one of the Any is in the form of an escaped string
	doc := `
{
	"properties": "{\n\"name\": \"config1\"\n,\n\"count\":20}\n",
	"nested": {
	  "properties": {
	    "name": "config2"
	  }
	}
}
`
	testSpec := testSpec{}
	err = Decode([]byte(doc), &testSpec)
	require.NoError(t, err)

	m := map[string]interface{}{}
	err = testSpec.Properties.Decode(&m)
	require.NoError(t, err)

	hash1 := Fingerprint(any)
	hash2 := Fingerprint(AnyBytes(any.Bytes()))
	require.Equal(t, hash1, hash2)

	hash3 := Fingerprint(AnyString(doc)) // different textual representation with escaped json string
	require.NotEqual(t, hash1, hash3)
}
