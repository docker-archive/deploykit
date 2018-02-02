package sh

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/docker/infrakit/pkg/callable/backend"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/util/exec"
)

var log = logutil.New("module", "cli/backend/sh")

func init() {
	backend.Register("sh", Sh, nil)
}

// Sh takes a list of optional parameters and returns an executable function that
// executes the content as a shell script
func Sh(scope scope.Scope, test bool, opt ...interface{}) (backend.ExecFunc, error) {

	return func(ctx context.Context, script string, params backend.Parameters, args []string) error {

		cmd := strings.Join(append([]string{"/bin/sh"}, args...), " ")
		log.Debug("sh", "cmd", cmd)

		run := exec.Command(cmd)
		run.InheritEnvs(true).StartWithHandlers(
			func(stdin io.Writer) error {
				_, err := stdin.Write([]byte(script))
				return err
			},
			func(stdout io.Reader) error {
				_, err := io.Copy(backend.GetWriter(ctx), stdout)
				return err
			},
			func(stderr io.Reader) error {
				_, err := io.Copy(os.Stderr, stderr)
				return err
			},
		)
		return run.Wait()
	}, nil
}
