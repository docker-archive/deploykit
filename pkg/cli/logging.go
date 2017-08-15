package cli

import (
	"github.com/spf13/pflag"

	"github.com/Sirupsen/logrus"
	logutil "github.com/docker/infrakit/pkg/log"
)

// DefaultLogLevel is the default log level value.
var DefaultLogLevel = len(logrus.AllLevels) - 2

// SetLogLevel adjusts the logrus level.
func SetLogLevel(level int) {
	if level > len(logrus.AllLevels)-1 {
		level = len(logrus.AllLevels) - 1
	} else if level < 0 {
		level = 0
	}
	logrus.SetLevel(logrus.AllLevels[level])
}

// Flags returns the set of logging flags
func Flags(o *logutil.Options) *pflag.FlagSet {
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
