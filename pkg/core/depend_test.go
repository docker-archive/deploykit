package core

import (
	"sort"
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/require"

	. "github.com/docker/infrakit/pkg/testing"
)

func TestFindSpecs0(t *testing.T) {

	spec := `
kind:        top
spiVersion:   top-version
metadata:
  name: top
properties:
  field1: value1
`
	s := types.Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &s))

	found := []string{}
	for _, f := range findSpecs(s) {
		found = append(found, f.Kind)
	}

	T(100).Infoln("list=", found)
	sort.Strings(found)
	require.Equal(t, []string{"top"}, found)
}

func TestFindSpecs1(t *testing.T) {

	spec := `
kind:        top
spiVersion:   top-version
metadata:
  name: top
properties:
  kind: nest1
  spiVersion: nest1-version
  metadata:
    name: nest1
  properties:
    kind: nest2
    spiVersion: nest2-version
    metadata:
      name: nest2
`
	s := types.Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &s))

	found := []string{}
	for _, f := range findSpecs(s) {
		found = append(found, f.Kind)
	}

	T(100).Infoln("list=", found)
	sort.Strings(found)
	require.Equal(t, []string{"nest1", "nest2", "top"}, found)
}

func TestFindSpecs2(t *testing.T) {

	spec := `
kind:        top
spiVersion:   top-version
metadata:
  name: top
properties:
  instance:
    kind: nest1
    spiVersion: nest1-version
    metadata:
      name: nest1
    properties:
      kind: nest2
      spiVersion: nest2-version
      metadata:
        name: nest2
  flavor:
    kind: nest3
    spiVersion: nest3-version
    metadata:
      name: nest3
    properties:
      kind: nest4
      spiVersion: nest4-version
      metadata:
        name: nest4
`
	s := types.Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &s))

	found := []string{}
	for _, f := range findSpecs(s) {
		found = append(found, f.Kind)
	}

	T(100).Infoln("list=", found)
	sort.Strings(found)
	require.Equal(t, []string{"nest1", "nest2", "nest3", "nest4", "top"}, found)
}

func TestFindSpecs3(t *testing.T) {

	spec := `
kind:        top
spiVersion:   top-version
metadata:
  name: top
properties:
  instance:
    kind: nest1
    spiVersion: nest1-version
    metadata:
      name: nest1
    properties:
      kind: nest2
      spiVersion: nest2-version
      metadata:
        name: nest2
      properties:
        - kind: nest5
          spiVersion: nest5-version
          metadata:
            name: nest5
        - kind: nest6
          spiVersion: nest6-version
          metadata:
            name: nest6
        - kind: nest7
          spiVersion: nest7-version
          metadata:
            name: nest7
  flavor:
    kind: nest3
    spiVersion: nest3-version
    metadata:
      name: nest3
    properties:
      kind: nest4
      spiVersion: nest4-version
      metadata:
        name: nest4
`
	s := types.Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &s))

	found := []string{}
	for _, f := range findSpecs(s) {
		found = append(found, f.Kind)
	}

	T(100).Infoln("list=", found)
	sort.Strings(found)
	require.Equal(t, []string{"nest1", "nest2", "nest3", "nest4", "nest5", "nest6", "nest7", "top"}, found)
}

func testDependency(t *testing.T, input string, expFound, expOrdered []string) {
	specs := []*types.Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(input), &specs))
	T(100).Infoln(specs)

	found := []string{}
	for _, f := range findSpecs(specs) {
		found = append(found, f.Metadata.Name)
	}

	T(100).Infoln("list=", found)
	sort.Strings(found)
	require.Equal(t, expFound, found)

	ordered, err := OrderByDependency(specs)
	require.NoError(t, err)

	found = []string{}
	for _, o := range ordered {
		found = append(found, o.Metadata.Name)
	}
	T(100).Infoln(found)
	require.Equal(t, expOrdered, found)
}

func TestDepedencyOrder1(t *testing.T) {

	testDependency(t, `
- kind:        top1C
  spiVersion:   top1-version
  metadata:
    name: top1N
  properties:
    instance:
      kind: nest1C
      spiVersion: nest1-version
      metadata:
        name: nest1N
      properties:
        kind: nest2C
        spiVersion: nest2-version
        metadata:
          name: nest2N
        properties:
          nest2Prop1: nest2Val1
          nest2Prop2: nest2Val2
    flavor:
      kind: nest3C
      spiVersion: nest3-version
      metadata:
        name: nest3N
- kind:        top2C
  spiVersion:   top2-version
  metadata:
    name: top2N
  depends:
    - kind: top1C
      name: top1N
- kind:        top3C
  spiVersion:   top3-version
  metadata:
    name: top3N
  depends:
    - kind: top2C
      name: top2N
- kind:        top4C
  spiVersion:   top4-version
  metadata:
    name: top4N
  depends:
    - kind: top3C
      name: top3N
`,
		[]string{"nest1N", "nest2N", "nest3N", "top1N", "top2N", "top3N", "top4N"},
		[]string{"top1N", "top2N", "top3N", "top4N"},
	)

	testDependency(t, `
- kind:        top1C
  spiVersion:   top1-version
  metadata:
    name: top1N
  properties:
- kind:        top2C
  spiVersion:   top2-version
  metadata:
    name: top2N
  depends:
    - kind: top3C
      name: top3N
- kind:        top3C
  spiVersion:   top3-version
  metadata:
    name: top3N
  depends:
    - kind: top4C
      name: top4N
- kind:        top4C
  spiVersion:   top4-version
  metadata:
    name: top4N
`,
		[]string{"top1N", "top2N", "top3N", "top4N"},
		[]string{"top4N", "top3N", "top2N", "top1N"},
	)

	testDependency(t, `
- kind:        top1C
  spiVersion:   top1-version
  metadata:
    name: top1N
  properties:
- kind:        top2C
  spiVersion:   top2-version
  metadata:
    name: top2N
  depends:
    - kind: top1C
      name: top1N
- kind:        top3C
  spiVersion:   top3-version
  metadata:
    name: top3N
  depends:
    - kind: top1C
      name: top1N
- kind:        top4C
  spiVersion:   top4-version
  metadata:
    name: top4N
  depends:
    - kind: top1C
      name: top1N
`,
		[]string{"top1N", "top2N", "top3N", "top4N"},
		[]string{"top1N", "top4N", "top3N", "top2N"},
	)

	testDependency(t, `
- kind:        pool
  spiVersion:   poolVersion
  metadata:
    name: pool1
  properties:
    instance:
      kind: ebs
      spiVersion: ebsVersion
      metadata:
        name: ebs1

- kind:        pool
  spiVersion:   poolVersion
  metadata:
    name: pool2
  properties:
    instance:
      kind: ebs
      spiVersion: ebsVersion
      metadata:
        name: ebs2

- kind:        group
  spiVersion:   groupVersion
  metadata:
    name: managers
  properties:
    instance:
       kind : instance
       spiVersion: instanceVersion
       metadata:
          name: instance-managers
    flavor:
       kind : flavor
       spiVersion: flavorVersion
       metadata:
          name: flavor-swarm-manager
  depends:
    - kind: pool
      name: pool1

- kind:        group
  spiVersion:   groupVersion
  metadata:
    name: workers
  properties:
    instance:
       kind : instance
       spiVersion: instanceVersion
       metadata:
          name: instance-workers
    flavor:
       kind : flavor
       spiVersion: flavorVersion
       metadata:
          name: flavor-swarm-worker
  depends:
    - kind: pool
      name: pool2
`,
		[]string{
			"ebs1",
			"ebs2",
			"flavor-swarm-manager",
			"flavor-swarm-worker",
			"instance-managers",
			"instance-workers",
			"managers",
			"pool1",
			"pool2",
			"workers",
		},
		[]string{
			"pool2",
			"workers",
			"pool1",
			"managers",
		},
	)

}

func testDependencyCycles(t *testing.T, input string, expFound []string) {
	specs := []*types.Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(input), &specs))
	T(100).Infoln(specs)

	found := []string{}
	for _, f := range findSpecs(specs) {
		found = append(found, f.Metadata.Name)
	}

	T(100).Infoln("list=", found)
	sort.Strings(found)
	require.Equal(t, expFound, found)

	_, err := OrderByDependency(specs)
	require.Error(t, err)
	T(100).Infoln(err)
}

func TestDepedencyOrder2Cycles(t *testing.T) {

	testDependencyCycles(t, `
- kind:        top1C
  spiVersion:   top1-version
  metadata:
    name: top1N
  properties:
    instance:
      kind: nest1C
      spiVersion: nest1-version
      metadata:
        name: nest1N
      properties:
        kind: nest2C
        spiVersion: nest2-version
        metadata:
          name: nest2N
        properties:
          nest2Prop1: nest2Val1
          nest2Prop2: nest2Val2
    flavor:
      kind: nest3C
      spiVersion: nest3-version
      metadata:
        name: nest3N
- kind:        top2C
  spiVersion:   top2-version
  metadata:
    name: top2N
  depends:
    - kind: top1C
      name: top1N
- kind:        top3C
  spiVersion:   top3-version
  metadata:
    name: top3N
  depends:
    - kind: top2C
      name: top2N
    - kind: top4C
      name: top4N
- kind:        top4C
  spiVersion:   top4-version
  metadata:
    name: top4N
  depends:
    - kind: top3C
      name: top3N
`,
		[]string{"nest1N", "nest2N", "nest3N", "top1N", "top2N", "top3N", "top4N"},
	)
}
