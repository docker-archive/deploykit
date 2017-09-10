package template

import (
	"encoding/json"
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"

	. "github.com/docker/infrakit/pkg/testing"
)

type testParameter struct {
	ParameterKey   string
	ParameterValue interface{}
}

type testResource struct {
	ResourceType      string
	LogicalResourceID string
	ResourceTypePtr   *string
}

type testCloud struct {
	Parameters   []testParameter
	Resources    []testResource
	ResourcePtrs []*testResource
	ResourceList []interface{}
}

func TestDeepCopyObject(t *testing.T) {
	resource := "disk"
	input := testCloud{
		Parameters: []testParameter{{ParameterKey: "foo", ParameterValue: "bar"}},
		Resources:  []testResource{{ResourceType: "test", ResourceTypePtr: &resource}},
	}

	copy, err := DeepCopyObject(input)
	require.NoError(t, err)
	require.Equal(t, input, copy)
	inputStr, err := ToJSON(input)
	require.NoError(t, err)
	copyStr, err := ToJSON(copy)
	require.NoError(t, err)
	require.Equal(t, inputStr, copyStr)
}

func TestQueryObjectEncodeDecode(t *testing.T) {

	param1 := testParameter{
		ParameterKey:   "key1",
		ParameterValue: "value1",
	}

	result, err := QueryObject("Parameters[?ParameterKey=='key1'] | [0]",
		testCloud{
			Parameters: []testParameter{
				param1,
				{
					ParameterKey:   "key2",
					ParameterValue: "value2",
				},
			},
		})
	require.NoError(t, err)

	encoded, err := ToJSON(param1)
	require.NoError(t, err)

	encoded2, err := ToJSON(result)
	require.Equal(t, encoded, encoded2)

	decoded, err := FromJSON(encoded)
	require.NoError(t, err)

	decoded2, err := FromJSON([]byte(encoded2))
	require.NoError(t, err)

	require.Equal(t, decoded, decoded2)

	decoded, err = FromJSON("[]")
	require.NoError(t, err)
	require.Equal(t, []interface{}{}, decoded)

	decoded, err = FromJSON(`{"foo":"bar"}`)
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{"foo": "bar"}, decoded)
}

func TestQueryObject(t *testing.T) {

	instanceType := testParameter{
		ParameterKey:   "instance-type",
		ParameterValue: "m2xlarge",
	}
	ami := testParameter{
		ParameterKey:   "ami",
		ParameterValue: "ami-1234",
	}

	vpc := testResource{
		ResourceType:      "AWS::EC2::VPC",
		LogicalResourceID: "Vpc",
	}
	subnet := testResource{
		ResourceType:      "AWS::EC2::Subnet",
		LogicalResourceID: "Subnet",
	}
	cloud := testCloud{
		Parameters: []testParameter{
			instanceType,
			ami,
		},
		Resources: []testResource{
			vpc,
			subnet,
		},
	}

	{
		result, err := QueryObject("Resources[?ResourceType=='AWS::EC2::Subnet'] | [0]", cloud)
		require.NoError(t, err)

		encoded, err := ToJSON(subnet)
		require.NoError(t, err)

		encoded2, err := ToJSON(result)
		require.Equal(t, encoded, encoded2)
	}
	{
		result, err := QueryObject("Resources", cloud)
		require.NoError(t, err)

		encoded, err := ToJSON([]testResource{vpc, subnet})
		require.NoError(t, err)

		encoded2, err := ToJSON(result)
		require.Equal(t, encoded, encoded2)
	}
	{
		result, err := QueryObject("Resources[?ResourceType=='AWS::EC2::VPC'] | [0].LogicalResourceID", cloud)
		require.NoError(t, err)
		require.Equal(t, "Vpc", result)
	}

}

func TestQueryObjectPtrs(t *testing.T) {

	vpc := testResource{
		ResourceType:      "AWS::EC2::VPC",
		LogicalResourceID: "Vpc",
	}
	subnet := testResource{
		ResourceType:      "AWS::EC2::Subnet",
		LogicalResourceID: "Subnet",
	}
	cloudTyped := testCloud{
		ResourcePtrs: []*testResource{
			&vpc,
			&subnet,
		},
	}

	doc, err := ToJSON(cloudTyped)
	require.NoError(t, err)
	cloud, err := FromJSON(doc)
	require.NoError(t, err)

	{
		result, err := QueryObject("ResourcePtrs[?ResourceType=='AWS::EC2::Subnet'] | [0]", cloudTyped)
		require.NoError(t, err)

		buff, _ := ToJSON(result)
		actual := testResource{}
		err = json.Unmarshal([]byte(buff), &actual)
		require.NoError(t, err)
		require.Equal(t, subnet, actual)
	}
	{
		result, err := QueryObject("ResourcePtrs[?ResourceType=='AWS::EC2::VPC'] | [0].LogicalResourceID", cloud)
		require.NoError(t, err)
		require.Equal(t, "Vpc", result)
	}

}

func TestQueryObjectInterfaces(t *testing.T) {

	vpc := testResource{
		ResourceType:      "AWS::EC2::VPC",
		LogicalResourceID: "Vpc",
	}
	subnet := testResource{
		ResourceType:      "AWS::EC2::Subnet",
		LogicalResourceID: "Subnet",
	}
	cloudTyped := testCloud{
		ResourceList: []interface{}{
			&vpc,
			&subnet,
		},
	}

	doc, err := ToJSON(cloudTyped)
	require.NoError(t, err)
	cloud, err := FromJSON(doc)
	require.NoError(t, err)

	{
		// query trhe map version of it
		result, err := QueryObject("ResourceList[?ResourceType=='AWS::EC2::Subnet'] | [0]", cloud)
		require.NoError(t, err)

		buff, _ := ToJSON(result)
		actual := testResource{}
		err = json.Unmarshal([]byte(buff), &actual)
		require.NoError(t, err)
		require.Equal(t, subnet, actual)
	}
	{
		result, err := QueryObject("ResourceList[?ResourceType=='AWS::EC2::VPC'] | [0].LogicalResourceID", cloudTyped)
		require.NoError(t, err)
		require.Equal(t, "Vpc", result)
	}

}

func strPtr(s string) *string {
	out := s
	return &out
}

func TestQueryObjectStrPtrs(t *testing.T) {

	vpc := testResource{
		ResourceTypePtr:   strPtr("AWS::EC2::VPC"),
		LogicalResourceID: "Vpc",
	}
	subnet := testResource{
		ResourceTypePtr:   strPtr("AWS::EC2::Subnet"),
		LogicalResourceID: "Subnet",
	}
	cloudTyped := testCloud{
		ResourceList: []interface{}{
			&vpc,
			&subnet,
		},
	}

	cloud, err := ToMap(cloudTyped)
	require.NoError(t, err)

	{
		// query the struct version
		result, err := QueryObject("ResourceList[?ResourceTypePtr=='AWS::EC2::Subnet'] | [0]", cloudTyped)
		require.NoError(t, err)
		require.Nil(t, result) // JMESPath cannot handle *string fields.
	}
	{
		// query the map version of it
		result, err := QueryObject("ResourceList[?ResourceTypePtr=='AWS::EC2::Subnet'] | [0]", cloud)
		require.NoError(t, err)

		buff, _ := ToJSON(result)
		actual := testResource{}
		err = json.Unmarshal([]byte(buff), &actual)
		require.NoError(t, err)
		require.Equal(t, subnet, actual)
	}
	{
		result, err := QueryObject("ResourceList[?ResourceTypePtr=='AWS::EC2::VPC'] | [0].LogicalResourceID", cloud)
		require.NoError(t, err)
		require.Equal(t, "Vpc", result)
	}

}

func TestMapEncodeDecode(t *testing.T) {

	vpc := testResource{
		ResourceTypePtr:   strPtr("AWS::EC2::VPC"),
		LogicalResourceID: "Vpc",
	}
	subnet := testResource{
		ResourceTypePtr:   strPtr("AWS::EC2::Subnet"),
		LogicalResourceID: "Subnet",
	}
	cloudTyped := testCloud{
		ResourceList: []interface{}{
			&vpc,
			&subnet,
		},
	}

	cloud, err := ToMap(cloudTyped)
	require.NoError(t, err)

	parsed := testCloud{}
	err = FromMap(cloud, &parsed)
	require.NoError(t, err)

	expect, err := ToMap(cloudTyped)
	require.NoError(t, err)

	actual, err := ToMap(parsed)
	require.NoError(t, err)

	require.Equal(t, expect, actual)
}

func TestIndexOf(t *testing.T) {
	require.Equal(t, -1, IndexOf("a", []string{"x", "y", "z"}))
	require.Equal(t, 1, IndexOf("y", []string{"x", "y", "z"}))
	require.Equal(t, -1, IndexOf(25, []string{"x", "y", "z"}))
	require.Equal(t, -1, IndexOf(25, 26))
	require.Equal(t, 1, IndexOf("y", []string{"x", "y", "z"}))
	require.Equal(t, 1, IndexOf("y", []interface{}{"x", "y", "z"}))
	require.Equal(t, 1, IndexOf(1, []interface{}{0, 1, 2}))
	require.Equal(t, 1, IndexOf("1", []interface{}{0, 1, 2}))
	require.Equal(t, 1, IndexOf(1, []interface{}{0, "1", 2}))
	require.Equal(t, -1, IndexOf("1", []interface{}{0, 1, 2}, true))  // strict case type must match
	require.Equal(t, 1, IndexOf("1", []interface{}{0, "1", 2}, true)) // strict case type must match
	require.Equal(t, -1, IndexOf(1, []interface{}{0, "1", 2}, true))  // strict case type must match

	v := "1"
	require.Equal(t, 1, IndexOf(&v, []interface{}{0, "1", 2}))
	require.Equal(t, 1, IndexOf(&v, []interface{}{0, &v, 2}, true))
	require.Equal(t, 1, IndexOf(&v, []interface{}{0, &v, 2}))

	a := "0"
	c := "2"
	require.Equal(t, 1, IndexOf("1", []*string{&a, &v, &c}))

	// This doesn't work because the type information is gone and we have just an address
	require.Equal(t, -1, IndexOf("1", []interface{}{0, &v, 2}))
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

func TestYAML(t *testing.T) {

	type instance struct {
		Type     string
		CPUCores int
		Memory   int
	}

	properties := types.AnyValueMust(instance{
		Type:     "large",
		CPUCores: 64,
		Memory:   512,
	})

	request := map[string]interface{}{
		"Plugin":     "instance",
		"Properties": properties,
	}

	yaml, err := ToYAML(request)
	require.NoError(t, err)

	T(100).Info(yaml)

	request2 := map[string]interface{}{
		"Plugin": "instance",
		"Properties": types.AnyString(`
{ "Type":"large", "CPUCores":64, "Memory":512 }
`),
	}

	yaml2, err := ToYAML(request2)
	require.NoError(t, err)
	T(100).Info(yaml2)

	require.Equal(t, yaml, yaml2)

	v, err := FromYAML(yaml)
	require.NoError(t, err)

	T(100).Info(v)

	v2, err := FromYAML(yaml2)
	require.NoError(t, err)

	T(100).Info(v2)

	require.Equal(t, v, v2)
}

func TestEscapeUnescape(t *testing.T) {

	text := `"\{\{.ID\}\}"`
	buff := Unescape([]byte(text))
	text2 := Escape(buff)
	require.Equal(t, string(text), string(text2))

	tt, err := NewTemplateFromBytes(buff, "", Options{})
	require.NoError(t, err)
	v, err := tt.Render(map[string]string{"ID": "hello"})
	require.NoError(t, err)
	require.Equal(t, `"hello"`, v)
}
