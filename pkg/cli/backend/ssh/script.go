package ssh

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/docker/infrakit/pkg/cli/backend"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/util/exec"
	"github.com/docker/infrakit/pkg/util/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var log = logutil.New("module", "cli/backend/ssh")

func init() {
	backend.Register("ssh", Script, func(flags *pflag.FlagSet) {
		flags.StringSlice("hostport", []string{}, "Host:port eg. localhost:22")
		flags.String("user", "", "username")
		flags.String("password", "", "password")
		flags.String("keyfile", "", "keyfile e.g. $HOME/.ssh/id_rsa")
	})
}

// Script takes a list of optional parameters and returns an executable function that
// executes the content as a shell script over ssh
// The args are user@host:port[,user@host:port] <auth> [password or keyfile]
// where auth = [ password | key | agent ]
func Script(scope scope.Scope, test bool, opt ...interface{}) (backend.ExecFunc, error) {

	return func(script string, cmd *cobra.Command, args []string) error {

		hostports, err := cmd.Flags().GetStringSlice("hostport")
		if err != nil {
			return err
		}
		user, err := cmd.Flags().GetString("user")
		if err != nil {
			return err
		}
		password, err := cmd.Flags().GetString("password")
		if err != nil {
			return err
		}
		keyfile, err := cmd.Flags().GetString("keyfile")
		if err != nil {
			return err
		}

		if test {
			fmt.Println("script options")
			for i, o := range opt {
				fmt.Printf("opt[%v] = %v\n", i, o)
			}
			fmt.Println("runtime cli flags")
			fmt.Printf("--hostport %v\n", hostports)
			fmt.Printf("--user %v\n", user)
			fmt.Printf("--password %v\n", password)
			fmt.Printf("--keyfile %v\n", keyfile)
			fmt.Println("runtime cli args")
			for i, a := range args {
				fmt.Printf("argv[%v] = %v\n", i, a)
			}
			fmt.Println("script")
			fmt.Print(script)
			return nil
		}

		var base ssh.Conn
		if keyfile != "" {
			base.Config = ssh.PublicKeyConfig(user, keyfile)
			log.Debug("using public key auth", "user", user, "keyfile", keyfile)
		} else if password != "" {
			base.Config = ssh.UsernamePasswordConfig(user, password)
			log.Debug("using password auth", "user", user)
		} else {
			base.Config = ssh.AgentConfig(user)
			log.Debug("using ssh agent auth", "user", user)
		}

		var wg sync.WaitGroup

		for _, hostport := range hostports {

			cl := base
			cl.Remote = ssh.HostPort(hostport)

			log.Debug("running", "remote", cl.Remote)

			wg.Add(1)
			go func() {
				defer wg.Done()

				exec, err := cl.Exec()
				if err != nil {
					log.Error("cannot connect", "remote", cl.Remote, "err", err)
					return
				}
				if err := execScript(exec, script, args); err != nil {
					log.Error("error", "remote", cl.Remote, "err", err)
					return
				}
			}()
		}

		wg.Wait()
		return nil
	}, nil
}

func execScript(impl exec.Interface, script string, args []string) error {
	cmd := strings.Join(append([]string{"/bin/sh"}, args...), " ")
	log.Debug("sh", "cmd", cmd)

	run := exec.Command(cmd)
	run.WithExec(impl).StartWithHandlers(
		func(stdin io.Writer) error {
			_, err := stdin.Write([]byte(script))
			return err
		},
		func(stdout io.Reader) error {
			_, err := io.Copy(os.Stdout, stdout)
			return err
		},
		func(stderr io.Reader) error {
			_, err := io.Copy(os.Stderr, stderr)
			return err
		},
	)
	return run.Wait()

}
