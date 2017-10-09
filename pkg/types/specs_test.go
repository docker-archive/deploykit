package types

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpecs(t *testing.T) {

	list, err := SpecsFromString(`
- kind: group
  metadata:
    name: workers
  properties:
    max: 10
    target: 5
- kind: group
  metadata:
    name: managers
  properties:
    target: 3
- kind: simulator/disk
  metadata:
    name: us-east1/disk1
  properties:
    iops: 100
    target: 5
- kind: simulator/disk
  metadata:
    name: us-east1/disk2
  properties:
    iops: 100
    target: 5
- kind: simulator/net
  metadata:
    name: us-east1/subnet1
  properties:
    cidr: 10.20.100.100/24
- kind: simulator/net
  metadata:
    name: us-east1/subnet3
  properties:
    cidr: 10.20.200.100/24
`)

	require.NoError(t, err)

	diff := list.Difference(Specs{})
	sort.Sort(list)
	sort.Sort(diff)
	require.EqualValues(t, list, diff)

	diff = (Specs{}).Difference(list)
	require.Equal(t, 0, len(diff))

	list2, err := SpecsFromString(`
[
  { "kind" : "group", "metadata" : { "name" : "managers" } },
  { "kind" : "group", "metadata" : { "name" : "workers" } }
]
`)
	require.NoError(t, err)
	diff = list.Difference(list2)
	sort.Sort(diff)
	sort.Sort(list)

	require.EqualValues(t, MustSpecs(SpecsFromString(`
- kind: simulator/disk
  metadata:
    name: us-east1/disk1
  properties:
    iops: 100
    target: 5
- kind: simulator/disk
  metadata:
    name: us-east1/disk2
  properties:
    iops: 100
    target: 5
- kind: simulator/net
  metadata:
    name: us-east1/subnet1
  properties:
    cidr: 10.20.100.100/24
- kind: simulator/net
  metadata:
    name: us-east1/subnet3
  properties:
    cidr: 10.20.200.100/24
`)), diff)

	require.Equal(t, 4, len(diff.Slice()))
}

func TestDelta1(t *testing.T) {

	old := MustSpecs(SpecsFromString(`
- kind: group
  metadata:
    name: workers
  properties:
    max: 10
    target: 5
- kind: group
  metadata:
    name: managers
  properties:
    target: 3
- kind: simulator/disk
  metadata:
    name: us-east1/disk1
  properties:
    iops: 100
    target: 5
- kind: simulator/disk
  metadata:
    name: us-east1/disk2
  properties:
    iops: 100
    target: 5
- kind: simulator/net
  metadata:
    name: us-east1/subnet1
  properties:
    cidr: 10.20.100.100/24
- kind: simulator/net
  metadata:
    name: us-east1/subnet3
  properties:
    cidr: 10.20.200.100/24
`))

	update := MustSpecs(SpecsFromString(`
- kind: group
  metadata:
    name: workers
  properties:
    max: 10
    target: 5
- kind: group
  metadata:
    name: managers
  properties:
    target: 3
- kind: simulator/disk
  metadata:
    name: us-east1/disk1
  properties:
    iops: 100
    target: 5
- kind: simulator/disk
  metadata:
    name: us-east1/disk2
  properties:
    iops: 100
    target: 5
- kind: simulator/net
  metadata:
    name: us-east1/subnet1
  properties:
    cidr: 10.20.100.100/24
- kind: simulator/net
  metadata:
    name: us-east1/subnet3
  properties:
    cidr: 10.20.200.100/24
`))

	add, remove, changes := old.Delta(update)
	require.Equal(t, Specs{}, add)
	require.Equal(t, Specs{}, remove)
	require.Equal(t, [][2]Spec{}, changes)

}

func TestDelta2(t *testing.T) {

	old := MustSpecs(SpecsFromString(`
- kind: group
  metadata:
    name: workers
  properties:
    max: 10
    target: 5
- kind: group
  metadata:
    name: managers
  properties:
    target: 3
- kind: simulator/net
  metadata:
    name: us-east1/subnet1
  properties:
    cidr: 10.20.100.100/24
- kind: simulator/net
  metadata:
    name: us-east1/subnet3
  properties:
    cidr: 10.20.200.100/24
`))

	update := MustSpecs(SpecsFromString(`
- kind: group
  metadata:
    name: workers
  properties:
    max: 100
    target: 5
- kind: group
  metadata:
    name: managers
  properties:
    target: 3
- kind: simulator/disk
  metadata:
    name: us-east1/disk1
  properties:
    iops: 100
    target: 5
- kind: simulator/net
  metadata:
    name: us-east1/subnet1
  properties:
    cidr: 10.20.100.100/24
`))

	add, remove, changes := old.Delta(update)
	require.Equal(t, MustSpecs(SpecsFromString(`
- kind: simulator/disk
  metadata:
    name: us-east1/disk1
  properties:
    iops: 100
    target: 5
`)), add)
	require.Equal(t, MustSpecs(SpecsFromString(`
- kind: simulator/net
  metadata:
    name: us-east1/subnet3
  properties:
    cidr: 10.20.200.100/24
`)), remove)
	require.Equal(t, [][2]Spec{
		{
			MustSpec(SpecFromString(`
kind: group
metadata:
    name: workers
properties:
    max: 10
    target: 5
`)),
			MustSpec(SpecFromString(`
kind: group
metadata:
    name: workers
properties:
    max: 100
    target: 5
`)),
		},
	}, changes)

}
