package types

import (
	"sort"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/require"

	. "github.com/docker/infrakit/pkg/testing"
)

func TestFindSpecs0(t *testing.T) {

	spec := `
class:        top
spiVersion:   top-version
metadata:
  name: top
properties:
  field1: value1
`
	s := Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &s))

	found := []string{}
	for _, f := range findSpecs(s) {
		found = append(found, f.Class)
	}

	T(100).Infoln("list=", found)
	sort.Strings(found)
	require.Equal(t, []string{"top"}, found)
}

func TestFindSpecs1(t *testing.T) {

	spec := `
class:        top
spiVersion:   top-version
metadata:
  name: top
properties:
  class: nest1
  spiVersion: nest1-version
  metadata:
    name: nest1
  properties:
    class: nest2
    spiVersion: nest2-version
    metadata:
      name: nest2
`
	s := Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &s))

	found := []string{}
	for _, f := range findSpecs(s) {
		found = append(found, f.Class)
	}

	T(100).Infoln("list=", found)
	sort.Strings(found)
	require.Equal(t, []string{"nest1", "nest2", "top"}, found)
}

func TestFindSpecs2(t *testing.T) {

	spec := `
class:        top
spiVersion:   top-version
metadata:
  name: top
properties:
  instance:
    class: nest1
    spiVersion: nest1-version
    metadata:
      name: nest1
    properties:
      class: nest2
      spiVersion: nest2-version
      metadata:
        name: nest2
  flavor:
    class: nest3
    spiVersion: nest3-version
    metadata:
      name: nest3
    properties:
      class: nest4
      spiVersion: nest4-version
      metadata:
        name: nest4
`
	s := Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &s))

	found := []string{}
	for _, f := range findSpecs(s) {
		found = append(found, f.Class)
	}

	T(100).Infoln("list=", found)
	sort.Strings(found)
	require.Equal(t, []string{"nest1", "nest2", "nest3", "nest4", "top"}, found)
}

func TestFindSpecs3(t *testing.T) {

	spec := `
class:        top
spiVersion:   top-version
metadata:
  name: top
properties:
  instance:
    class: nest1
    spiVersion: nest1-version
    metadata:
      name: nest1
    properties:
      class: nest2
      spiVersion: nest2-version
      metadata:
        name: nest2
      properties:
        - class: nest5
          spiVersion: nest5-version
          metadata:
            name: nest5
        - class: nest6
          spiVersion: nest6-version
          metadata:
            name: nest6
        - class: nest7
          spiVersion: nest7-version
          metadata:
            name: nest7
  flavor:
    class: nest3
    spiVersion: nest3-version
    metadata:
      name: nest3
    properties:
      class: nest4
      spiVersion: nest4-version
      metadata:
        name: nest4
`
	s := Spec{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &s))

	found := []string{}
	for _, f := range findSpecs(s) {
		found = append(found, f.Class)
	}

	T(100).Infoln("list=", found)
	sort.Strings(found)
	require.Equal(t, []string{"nest1", "nest2", "nest3", "nest4", "nest5", "nest6", "nest7", "top"}, found)
}

func testDependency(t *testing.T, input string, expFound, expOrdered []string) {
	specs := []*Spec{}
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
- class:        top1C
  spiVersion:   top1-version
  metadata:
    name: top1N
  properties:
    instance:
      class: nest1C
      spiVersion: nest1-version
      metadata:
        name: nest1N
      properties:
        class: nest2C
        spiVersion: nest2-version
        metadata:
          name: nest2N
        properties:
          nest2Prop1: nest2Val1
          nest2Prop2: nest2Val2
    flavor:
      class: nest3C
      spiVersion: nest3-version
      metadata:
        name: nest3N
- class:        top2C
  spiVersion:   top2-version
  metadata:
    name: top2N
  depends:
    - class: top1C
      name: top1N
- class:        top3C
  spiVersion:   top3-version
  metadata:
    name: top3N
  depends:
    - class: top2C
      name: top2N
- class:        top4C
  spiVersion:   top4-version
  metadata:
    name: top4N
  depends:
    - class: top3C
      name: top3N
`,
		[]string{"nest1N", "nest2N", "nest3N", "top1N", "top2N", "top3N", "top4N"},
		[]string{"top1N", "top2N", "top3N", "top4N"},
	)

	testDependency(t, `
- class:        top1C
  spiVersion:   top1-version
  metadata:
    name: top1N
  properties:
- class:        top2C
  spiVersion:   top2-version
  metadata:
    name: top2N
  depends:
    - class: top3C
      name: top3N
- class:        top3C
  spiVersion:   top3-version
  metadata:
    name: top3N
  depends:
    - class: top4C
      name: top4N
- class:        top4C
  spiVersion:   top4-version
  metadata:
    name: top4N
`,
		[]string{"top1N", "top2N", "top3N", "top4N"},
		[]string{"top4N", "top3N", "top2N", "top1N"},
	)

	testDependency(t, `
- class:        top1C
  spiVersion:   top1-version
  metadata:
    name: top1N
  properties:
- class:        top2C
  spiVersion:   top2-version
  metadata:
    name: top2N
  depends:
    - class: top1C
      name: top1N
- class:        top3C
  spiVersion:   top3-version
  metadata:
    name: top3N
  depends:
    - class: top1C
      name: top1N
- class:        top4C
  spiVersion:   top4-version
  metadata:
    name: top4N
  depends:
    - class: top1C
      name: top1N
`,
		[]string{"top1N", "top2N", "top3N", "top4N"},
		[]string{"top1N", "top4N", "top3N", "top2N"},
	)

	testDependency(t, `
- class:        pool
  spiVersion:   poolVersion
  metadata:
    name: pool1
  properties:
    instance:
      class: ebs
      spiVersion: ebsVersion
      metadata:
        name: ebs1

- class:        pool
  spiVersion:   poolVersion
  metadata:
    name: pool2
  properties:
    instance:
      class: ebs
      spiVersion: ebsVersion
      metadata:
        name: ebs2

- class:        group
  spiVersion:   groupVersion
  metadata:
    name: managers
  properties:
    instance:
       class : instance
       spiVersion: instanceVersion
       metadata:
          name: instance-managers
    flavor:
       class : flavor
       spiVersion: flavorVersion
       metadata:
          name: flavor-swarm-manager
  depends:
    - class: pool
      name: pool1

- class:        group
  spiVersion:   groupVersion
  metadata:
    name: workers
  properties:
    instance:
       class : instance
       spiVersion: instanceVersion
       metadata:
          name: instance-workers
    flavor:
       class : flavor
       spiVersion: flavorVersion
       metadata:
          name: flavor-swarm-worker
  depends:
    - class: pool
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
	specs := []*Spec{}
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
- class:        top1C
  spiVersion:   top1-version
  metadata:
    name: top1N
  properties:
    instance:
      class: nest1C
      spiVersion: nest1-version
      metadata:
        name: nest1N
      properties:
        class: nest2C
        spiVersion: nest2-version
        metadata:
          name: nest2N
        properties:
          nest2Prop1: nest2Val1
          nest2Prop2: nest2Val2
    flavor:
      class: nest3C
      spiVersion: nest3-version
      metadata:
        name: nest3N
- class:        top2C
  spiVersion:   top2-version
  metadata:
    name: top2N
  depends:
    - class: top1C
      name: top1N
- class:        top3C
  spiVersion:   top3-version
  metadata:
    name: top3N
  depends:
    - class: top2C
      name: top2N
    - class: top4C
      name: top4N
- class:        top4C
  spiVersion:   top4-version
  metadata:
    name: top4N
  depends:
    - class: top3C
      name: top3N
`,
		[]string{"nest1N", "nest2N", "nest3N", "top1N", "top2N", "top3N", "top4N"},
	)
}
