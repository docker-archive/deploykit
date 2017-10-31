package print

import (
	"fmt"
	"strings"

	"github.com/docker/infrakit/pkg/cli/backend"
	"github.com/docker/infrakit/pkg/run/scope"
)

func init() {
	backend.Register("print", Print)
}

// Print takes a list of optional parameters and returns an executable function that prints
// arg0 is the prefix. it's optional
func Print(scope scope.Scope, opt ...interface{}) (backend.ExecFunc, error) {

	prefix := ""
	if len(opt) > 0 {
		prefix = fmt.Sprintf("%v", opt[0])
	}
	return func(script string) error {
		if prefix == "" {
			fmt.Println(script)
			return nil
		}

		lines := strings.Split(script, "\n")
		fmt.Println(prefix + strings.Join(lines, "\n"+prefix))
		return nil
	}, nil
}
