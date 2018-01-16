package types

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func mustSpec(s types.Spec, err error) types.Spec {
	if err != nil {
		panic(err)
	}
	return s
}

func specFromString(s string) (types.Spec, error) {
	v, err := types.AnyYAML([]byte(s))
	if err != nil {
		return types.Spec{}, err
	}
	spec := types.Spec{}
	err = v.Decode(&spec)
	return spec, err
}

func TestWriteProperties(t *testing.T) {
	p := Properties{
		List: (*ListSourceUnion)(types.AnyValueMust([]instance.Description{
			{ID: instance.ID("host1")},
			{ID: instance.ID("host2")},
		})),
		Instance: PluginSpec{
			Plugin:     plugin.Name("simulator/compute"),
			Properties: types.AnyValueMust("test"),
		},
	}

	buff, err := types.AnyValueMust(p).MarshalYAML()
	require.NoError(t, err)

	p2 := Properties{}
	err = types.AnyYAMLMust(buff).Decode(&p2)
	require.NoError(t, err)

	list1, err := p.List.InstanceDescriptions()
	require.NoError(t, err)

	list2, err := p2.List.InstanceDescriptions()
	require.NoError(t, err)

	require.EqualValues(t, list2, list1)
}

func TestParseProperties(t *testing.T) {

	spec := mustSpec(specFromString(`
kind: enrollment
metadata:
  name: nfs
properties:
  List:
    - ID: host1
    - ID: host2
    - ID: host3
    - ID: host4
  Instance:
    Plugin: us-east/nfs-authorizer
    Properties:
      Id: \{\{ .ID \}\}
`))

	p := Properties{}
	err := spec.Properties.Decode(&p)
	require.NoError(t, err)

	list, err := p.List.InstanceDescriptions()
	require.NoError(t, err)

	_, err = p.List.GroupPlugin()
	require.Error(t, err)

	require.EqualValues(t, []instance.Description{
		{ID: instance.ID("host1")},
		{ID: instance.ID("host2")},
		{ID: instance.ID("host3")},
		{ID: instance.ID("host4")},
	}, list)
}

func TestParsePropertiesWithGroup(t *testing.T) {

	spec := mustSpec(specFromString(`
kind: enrollment
metadata:
  name: nfs
properties:
  List: us-east/workers
  Instance:
    Plugin: us-east/nfs-authorizer
    Properties:
      Id: \{\{ .ID \}\}
`))

	p := Properties{}
	err := spec.Properties.Decode(&p)
	require.NoError(t, err)

	_, err = p.List.InstanceDescriptions()
	require.Error(t, err)

	g, err := p.List.GroupPlugin()
	require.NoError(t, err)
	require.Equal(t, plugin.Name("us-east/workers"), g)

	spec2 := types.Spec{}
	require.NoError(t, types.AnyString(`
{
  "kind": "enrollment",
  "metadata" : {
    "name" : "nfs"
  },
  "options" : {
    "SourceKeySelector" : "\\{\\{.ID\\}\\}"
  },
  "properties" : {
    "List" : [
      { "ID" : "h1" },
      { "ID" : "h2" }
    ],
    "Instance" : {
      "Plugin" : "us-east/nfs-authorizer",
      "Properties" : {
        "ID" : "\\{\\{.ID\\}\\}"
      }
    }
  }
}
`).Decode(&spec2))

	tt, err := TemplateFrom(spec2.Options.Bytes())
	require.NoError(t, err)

	obj := map[string]string{"ID": "hello"}
	view, err := tt.Render(obj)
	require.NoError(t, err)
	require.Equal(t, "{\n    \"SourceKeySelector\" : \"hello\"\n  }", view)

	type properties struct {
		Instance struct {
			Properties struct {
				ID string
			}
		}
	}

	pp := properties{}
	tt, err = TemplateFrom(spec2.Properties.Bytes())
	require.NoError(t, err)
	view, err = tt.Render(obj)
	require.NoError(t, err)
	require.NoError(t, types.AnyString(view).Decode(&pp))
	require.Equal(t, "hello", pp.Instance.Properties.ID)
}

func TestValidate(t *testing.T) {
	// Valid Options
	o := Options{
		SyncInterval:             types.FromDuration(time.Duration(10 * time.Second)),
		SourceParseErrPolicy:     SourceParseErrorDisableDestroy,
		EnrollmentParseErrPolicy: EnrolledParseErrorDisableProvision,
	}
	require.NoError(t, o.Validate(PluginInit))
	require.NoError(t, o.Validate(PluginCommit))
	// Invalid SyncInterval, only an error for init
	o = Options{
		SyncInterval:             types.FromDuration(time.Duration(-1 * time.Second)),
		SourceParseErrPolicy:     SourceParseErrorDisableDestroy,
		EnrollmentParseErrPolicy: EnrolledParseErrorDisableProvision,
	}
	err := o.Validate(PluginInit)
	require.Equal(t, fmt.Errorf("SyncInterval must be greater than 0"), err)
	require.NoError(t, o.Validate(PluginCommit))
	// Invalid SourceParseErrPolicy
	o = Options{
		SyncInterval:             types.FromDuration(time.Duration(10 * time.Second)),
		SourceParseErrPolicy:     "bogus-SourceParseErrPolicy",
		EnrollmentParseErrPolicy: EnrolledParseErrorDisableProvision,
	}
	for _, phase := range []PluginPhase{PluginInit, PluginCommit} {
		err := o.Validate(phase)
		require.Error(t, err)
		require.Equal(t,
			fmt.Errorf("SourceParseErrPolicy value 'bogus-SourceParseErrPolicy' is not supported, valid values: %v",
				[]string{SourceParseErrorEnableDestroy, SourceParseErrorDisableDestroy}),
			err)
	}
	// Invalid EnrollmentParseErrPolicy
	o = Options{
		SyncInterval:             types.FromDuration(time.Duration(10 * time.Second)),
		SourceParseErrPolicy:     SourceParseErrorDisableDestroy,
		EnrollmentParseErrPolicy: "bogus-EnrollmentParseErrPolicy",
	}
	for _, phase := range []PluginPhase{PluginInit, PluginCommit} {
		err := o.Validate(phase)
		require.Error(t, err)
		require.Equal(t,
			fmt.Errorf("EnrollmentParseErrPolicy value 'bogus-EnrollmentParseErrPolicy' is not supported, valid values: %v",
				[]string{EnrolledParseErrorEnableProvision, EnrolledParseErrorDisableProvision}),
			err)
	}
}
