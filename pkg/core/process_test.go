package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/fsm"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	. "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestOverlay(t *testing.T) {

	// We test for overlaying two structures where one struct is the
	// initial default values while the second any is decoded using that
	// initial value and new values and overrides are decoded as a result.

	v1 := map[string]interface{}{
		"x": 1.,
		"yy": map[string]interface{}{
			"z": 2.,
		},
	}

	text := `
x: 1.
yy:
 z: 2.
`

	var v2 interface{}
	types.AnyYAMLMust([]byte(text)).Decode(&v2)

	require.Equal(t, v2, v1)

	text = `
kk: kk
yy:
 z: 24.
 zz: test
`
	types.AnyYAMLMust([]byte(text)).Decode(&v1)

	var expected interface{}
	types.AnyYAMLMust([]byte(`
kk: kk
x: 1.
yy:
 z: 24.
 zz: test
`)).Decode(&expected)
	require.Equal(t, expected, v1)
}

func TestNormalizeSpecs(t *testing.T) {

	root := testGetRootURL(t)
	url := filepath.Join(root, "simple.yml")

	source, buff, err := SpecsFromURL(url)
	require.NoError(t, err)
	require.Equal(t, url, source)
	require.True(t, len(buff) > 0)

	T(100).Infoln(string(buff))

	ordered, err := NormalizeSpecs(url, buff)
	require.NoError(t, err)
	require.Equal(t, 2, len(ordered))

	for _, spec := range ordered {

		require.NotNil(t, spec.Template)
		require.True(t, spec.Template.Absolute())
		T(100).Infoln(spec.Template.String(), "root=", root)
		require.True(t, strings.Contains(spec.Template.String(), root))
	}
}

func testGetRootURL(t *testing.T) string {
	dir, err := os.Getwd()
	require.NoError(t, err)

	testdata := filepath.Join(dir, "testdata")
	return "file://" + testdata
}

func testRenderProperties(t *testing.T, text string, depends map[string]interface{}, scope scope.Scope,
	assert func(*types.Any)) {

	url := testGetRootURL(t)
	ordered, err := NormalizeSpecs(url+"/fake.yml", []byte(text))
	require.NoError(t, err)

	any, err := renderProperties(&types.Object{Spec: *ordered[0]}, fsm.ID(0), depends, scope)
	require.NoError(t, err)

	assert(any)
}

func TestRenderProperties(t *testing.T) {

	// Case - no template, just properties. Note the escapes here.
	testRenderProperties(t, `
- kind:        instance-aws/ec2-instance
  version:   instance/v0.1.0
  metadata:
    name: host1
    tags:
      role:    worker
      project: test
  properties:
    AttachVolumeInputs:
      Device: /dev/sdf
      InstanceId: "{{ var \"instanceId\" }}"
      VolumeId: "{{ var \"volumeId\" }}"
    RunInstancesInput:
      ImageId: ami-1234
      InstanceType: t2-small
      KeyName: "{{ var \"metadata/tags/role\" }}-key"
      MaxCount: 1
      MinCount: 1
  options:
    region: us-west-1
    stack:  test
`,
		map[string]interface{}{
			"volumeId":   "vol-1234",
			"instanceId": "uid-1234",
		},
		scope.Nil,
		func(properties *types.Any) {

			m := map[string]interface{}{}
			require.NoError(t, properties.Decode(&m))

			require.Equal(t, map[string]interface{}{
				"AttachVolumeInputs": map[string]interface{}{
					"Device":     "/dev/sdf",
					"InstanceId": "uid-1234",
					"VolumeId":   "vol-1234",
				},
				"RunInstancesInput": map[string]interface{}{
					"ImageId":      "ami-1234",
					"InstanceType": "t2-small",
					"KeyName":      "worker-key",
					"MaxCount":     1.,
					"MinCount":     1.,
				},
			}, m)

		})
	// Case - has template.  No properties
	testRenderProperties(t, `
- kind:        instance-aws/ec2-instance
  version:   instance/v0.1.0
  metadata:
    name: host1
    tags:
      role:    worker
      project: test
  template: aws-instance-template.yml
  options:
    region: us-west-1
    stack:  test
`,
		map[string]interface{}{},
		scope.Nil,
		func(properties *types.Any) {

			m := map[string]interface{}{}
			require.NoError(t, properties.Decode(&m))

			// we expect all the defaults to be used that are defined in the template, because there isn't a Properties
			// section to provide any overrides.

			require.Equal(t, "default-key", query(t, "RunInstancesInput.KeyName", m))
			require.Equal(t, "default-image-id", query(t, "RunInstancesInput.ImageId", m))
			require.Equal(t, "default-instance-type", query(t, "RunInstancesInput.InstanceType", m))
			require.Equal(t, true, query(t, "RunInstancesInput.NetworkInterfaces[0].AssociatePublicIpAddress", m))
			require.Equal(t, 1., query(t, "RunInstancesInput.MaxCount", m))
		})

	// Case - has template, with properties override
	testRenderProperties(t, `
- kind:        instance-aws/ec2-instance
  version:   instance/v0.1.0
  metadata:
    name: host1
    tags:
      role:    worker
      project: test
  template: aws-instance-template.yml
  properties:
    instanceType: m2-xlarge
    imageId:      ami-12345
    key: mySSH
  options:
    region: us-west-1
    stack:  test
`,
		map[string]interface{}{},
		scope.Nil,
		func(properties *types.Any) {

			m := map[string]interface{}{}
			require.NoError(t, properties.Decode(&m))

			// Here we have a Properties section to provide overrides

			require.Equal(t, "mySSH", query(t, "RunInstancesInput.KeyName", m))
			require.Equal(t, "ami-12345", query(t, "RunInstancesInput.ImageId", m))
			require.Equal(t, "m2-xlarge", query(t, "RunInstancesInput.InstanceType", m))
			require.Equal(t, true, query(t, "RunInstancesInput.NetworkInterfaces[0].AssociatePublicIpAddress", m))
			require.Equal(t, 1., query(t, "RunInstancesInput.MaxCount", m))
		})

}

func query(t *testing.T, exp string, v interface{}) interface{} {
	q, err := template.QueryObject(exp, v)
	require.NoError(t, err)
	return q
}

func TestProcess(t *testing.T) {

	text := `
- kind:        instance-aws/ec2-instance
  version:   instance/v0.1.0
  metadata:
    name: workers
    tags:
      role:    worker
      project: test
  template: aws-instance-template.yml
  properties:
    instanceType: m2-xlarge
    imageId:      ami-12345
    key: mySSH
  options:
    region: us-west-1
    stack:  test
`

	url := testGetRootURL(t)
	ordered, err := NormalizeSpecs(url+"/fake.yml", []byte(text))
	require.NoError(t, err)

	// states
	const (
		unavailable fsm.Index = iota
		specified
		available
	)

	// signals
	const (
		exception fsm.Signal = iota
		found
		create
	)

	store := NewObjects(func(o *types.Object) []interface{} {
		return []interface{}{o.Metadata.Name, o.Metadata.Identity.ID}
	})

	createArgs := make(chan *types.Any)
	createSpec := make(chan types.Spec)

	proc, err := NewProcess(
		func(p *Process) (*fsm.Spec, error) {
			return fsm.Define(
				fsm.State{
					Index: specified,
					Transitions: map[fsm.Signal]fsm.Index{
						found:  available,
						create: available,
					},
					Actions: map[fsm.Signal]fsm.Action{
						create: p.Constructor,
					},
				},
				fsm.State{
					Index: available,
					Transitions: map[fsm.Signal]fsm.Index{
						exception: unavailable,
					},
				},
				fsm.State{
					Index: unavailable,
				},
			)
		},

		ProcessDefinition{
			Spec: ordered[0],
			Constructor: func(spec types.Spec, properties *types.Any) (*types.Identity, *types.Any, error) {
				createArgs <- properties
				createSpec <- spec
				return &types.Identity{ID: "new"}, nil, nil
			},
		},

		store,
		scope.Nil,
	)

	require.NoError(t, err)
	require.NotNil(t, proc)

	err = proc.Start(fsm.Wall(time.Tick(100 * time.Millisecond)))
	require.NoError(t, err)

	// Add one
	instance := proc.Instances().Add(specified)

	err = instance.Signal(create)
	require.NoError(t, err)

	properties := <-createArgs
	spec := <-createSpec

	T(100).Infoln(properties.String())

	// should be 0 for the id -- the type is bad
	require.Equal(t, float64(0), types.Get(types.PathFromString("AttachVolumeInputs/InstanceId"), properties))
	require.Equal(t, nil, types.Get(types.PathFromString("AttachVolumeInputs/VolumeId"), properties))
	require.Equal(t, "mySSH", types.Get(types.PathFromString("RunInstancesInput/KeyName"), properties))
	require.Equal(t, "ami-12345", types.Get(types.PathFromString("RunInstancesInput/ImageId"), properties))
	require.Equal(t, "m2-xlarge", types.Get(types.PathFromString("RunInstancesInput/InstanceType"), properties))

	T(100).Infoln(spec)
	require.Equal(t, "instance-aws/ec2-instance", types.Get(types.PathFromString("Kind"), spec))

	// we should have 1 instance in the available state
	require.Equal(t, 1, proc.Instances().CountByState(available))

	// get the object
	obj := proc.Object(instance)
	require.NotNil(t, obj)
	require.Equal(t, "new", obj.Metadata.Identity.ID)
}
