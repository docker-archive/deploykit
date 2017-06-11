package instance

import (
	"testing"

	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/spi/instance"
	. "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/types"
)

func TestSetDifferences(t *testing.T) {

	a := instance.Description{
		ID: instance.ID("a"),
		Tags: map[string]string{
			"group": "A",
			"label": "AA",
		},
		Properties: types.AnyString(`{"a":"A"}`),
	}
	b := instance.Description{
		ID: instance.ID("b"),
		Tags: map[string]string{
			"group": "B",
			"label": "BB",
		},
		Properties: types.AnyString(`{"b":"B"}`),
	}
	c := instance.Description{
		ID: instance.ID("c"),
		Tags: map[string]string{
			"group": "C",
			"label": "CC",
		},
		Properties: types.AnyString(`{"c":"C"}`),
	}
	aa := instance.Description{
		ID: instance.ID("aa"),
		Tags: map[string]string{
			"group": "A",
		},
		Properties: types.AnyString(`{"a":"A"}`),
	}
	T(100).Infoln(aa)
	last := mapset.NewSet()

	current := mapset.NewSet()
	current.Add(a.ID)
	current.Add(b.ID)
	current.Add(c.ID)

	lost := last.Difference(current)
	found := current.Difference(last)

	T(100).Infoln("lost:", lost)
	T(100).Infoln("found:", found)

	last = current
	current = mapset.NewSet()
	current.Add(aa.ID)
	current.Add(b.ID)
	current.Add(c.ID)

	lost = last.Difference(current)
	found = current.Difference(last)

	T(100).Infoln("lost:", lost)
	T(100).Infoln("found:", found)
}
