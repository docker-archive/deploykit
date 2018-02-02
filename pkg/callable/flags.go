package callable

import (
	"fmt"

	"github.com/docker/infrakit/pkg/callable/backend"
	"github.com/spf13/pflag"
)

// ParametersFromFlags returns a Parameter implementation from CLI flags.
func ParametersFromFlags(flags *pflag.FlagSet) backend.Parameters {
	return &parameters{flags}
}

type parameters struct {
	*pflag.FlagSet
}

// SetParameter implements pkg/callable/backend/Parameters
func (p parameters) SetParameter(name string, value interface{}) error {
	return p.Set(name, fmt.Sprintf("%v", value))
}
