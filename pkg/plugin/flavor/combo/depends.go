package combo

import (
	"github.com/docker/infrakit/pkg/run/depends"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/types"
)

func init() {
	depends.Register("combo", types.InterfaceSpec(flavor.InterfaceSpec), ResolveDependencies)
}

// ResolveDependencies returns a list of dependencies by parsing the opaque Properties blob.
// Do not include self -- only the children / dependent components.
func ResolveDependencies(spec types.Spec) (depends.Runnables, error) {
	if spec.Properties == nil {
		return nil, nil
	}

	flavorSpecs := Spec{}
	err := spec.Properties.Decode(&flavorSpecs)
	if err != nil {
		return nil, err
	}

	runnables := depends.Runnables{}
	for _, flavorSpec := range flavorSpecs {
		included := depends.RunnableFrom(flavorSpec.Plugin)
		runnables = append(runnables, included)
	}
	return runnables, nil
}
