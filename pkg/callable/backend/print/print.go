package print

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/infrakit/pkg/callable/backend"
	"github.com/docker/infrakit/pkg/run/scope"
)

func init() {
	backend.Register("print", Print, nil)
	backend.Register("doc", Print, nil)
	backend.Register("text", Print, nil)
}

// Print takes a list of optional parameters and returns an executable function that prints
// arg0 is the prefix. it's optional
func Print(scope scope.Scope, test bool, opt ...interface{}) (backend.ExecFunc, error) {

	prefix := ""
	if len(opt) > 0 {
		prefix = fmt.Sprintf("%v", opt[0])
	}
	return func(ctx context.Context, script string, parameters backend.Parameters, args []string) error {

		out := backend.GetWriter(ctx)

		if prefix == "" {
			fmt.Fprintln(out, script)
			return nil
		}

		lines := strings.Split(script, "\n")
		fmt.Fprintln(out, prefix+strings.Join(lines, "\n"+prefix))
		return nil
	}, nil
}
