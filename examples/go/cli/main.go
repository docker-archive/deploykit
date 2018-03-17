package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/docker/infrakit/pkg/cli/playbook"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	// backends that are supported
	_ "github.com/docker/infrakit/pkg/callable/backend/http"
	_ "github.com/docker/infrakit/pkg/callable/backend/print"
	_ "github.com/docker/infrakit/pkg/callable/backend/sh"
	_ "github.com/docker/infrakit/pkg/callable/backend/ssh"
	_ "github.com/docker/infrakit/pkg/callable/backend/vmwscript"
)

func init() {
	logutil.Configure(&logutil.ProdDefaults)
}

func makeURL(s string) string {
	if strings.Index(s, "://") > 0 {
		return s
	}

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return "file://" + path.Join(wd, s)
}

func main() {

	prog := path.Base(os.Args[0])
	if len(os.Args) < 2 {
		fmt.Printf("Usage %v <url> [flags...]", prog)
		os.Exit(-1)
	}

	// Commands from playbooks
	playbooks, err := playbook.NewModules(
		scope.Nil,
		playbook.Modules{
			playbook.Op(prog): playbook.SourceURL(makeURL(os.Args[1])),
		},
		os.Stdin, template.Options{})

	if err != nil {
		panic(err)
	}

	commands, err := playbooks.List()
	if err != nil {
		panic(err)
	}

	// we expect only one top level command because there's only one playbook url
	if len(commands) == 0 {
		panic("unable to make commands out of the playbook " + os.Args[1])
	}

	top := commands[0]

	top.PersistentFlags().AddFlagSet(logFlags(&logutil.ProdDefaults))
	top.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		logutil.Configure(&logutil.ProdDefaults)
		return nil
	}
	// Don't print usage text for any error returned from a RunE function.
	// Only print it when explicitly requested.
	top.SilenceUsage = true
	// Don't automatically print errors returned from a RunE function.
	// They are returned from cmd.Execute() below and we print it ourselves.
	top.SilenceErrors = true

	// consume the first two args and let the command process the rest
	top.SetArgs(os.Args[2:])
	top.Execute()
}

func logFlags(o *logutil.Options) *pflag.FlagSet {
	f := pflag.NewFlagSet("logging", pflag.ExitOnError)
	f.IntVar(&o.Level, "log", o.Level, "log level")
	f.IntVar(&o.DebugV, "log-debug-V", o.DebugV, "log debug verbosity level. 0=logs all")
	f.BoolVar(&o.Stdout, "log-stdout", o.Stdout, "log to stdout")
	f.BoolVar(&o.CallFunc, "log-caller", o.CallFunc, "include caller function")
	f.BoolVar(&o.CallStack, "log-stack", o.CallStack, "include caller stack")
	f.StringVar(&o.Format, "log-format", o.Format, "log format: logfmt|term|json")
	f.BoolVar(&o.DebugMatchExclude, "log-debug-match-exclude", false, "True to exclude; otherwise only include matches")
	f.StringSliceVar(&o.DebugMatchKeyValuePairs, "log-debug-match", []string{},
		"debug mode only -- select records with any of the k=v pairs")
	return f
}
