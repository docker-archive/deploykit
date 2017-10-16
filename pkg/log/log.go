package log

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/run/local"
	"gopkg.in/inconshreveable/log15.v2"
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

// Options capture the logging configuration
type Options struct {
	Level     int
	Stdout    bool
	Format    string
	CallFunc  bool
	CallStack bool

	// DebugMatchKeyValuePairs is the '=' delimited kv pair for filtering contexts in records for DEBUG
	DebugMatchKeyValuePairs []string

	// DebugMatchExclude excludes the record if any of the context params matches those in the MatchKeyValuePairs
	DebugMatchExclude bool

	// DebugV is a value of verbosity for checking debug records where there's a context param of type DebugV
	DebugV int
}

// V is the Verbosity level.  To set a verbosity level, just put a log.V(100) in the context
// (e.g. .Debug(msg, "key", log.V(100))
type V int

func mustFirst(v int, err error) int {
	if err != nil {
		panic(err)
	}
	return v
}

// DevDefaults is the default options for development
var DevDefaults = Options{
	Level:     mustFirst(strconv.Atoi(local.Getenv("INFRAKIT_LOG_DEV_LEVEL", "4"))),
	Stdout:    false,
	Format:    local.Getenv("INFRAKIT_LOG_DEV_FORMAT", "term"),
	CallStack: true,
}

// ProdDefaults is the default options for production
var ProdDefaults = Options{
	Level:    4,
	Stdout:   false,
	Format:   "term",
	CallFunc: true,
}

func init() {
	Configure(&DevDefaults)
}

// New returns a logger of given context
func New(ctx ...interface{}) log15.Logger {
	return log15.Root().New(ctx...)
}

// Root returns the process's root logger
func Root() log15.Logger {
	return log15.Root()
}

// Configure configures the logging
func Configure(options *Options) {

	SetLogLevel(options.Level)

	var f log15.Format
	switch options.Format {
	case "term":
		f = log15.TerminalFormat()
	case "json":
		f = log15.JsonFormatEx(true, true)
	case "logfmt":
		fallthrough
	default:
		f = log15.LogfmtFormat()
	}

	var h log15.Handler
	if options.Stdout {
		h = log15.StreamHandler(os.Stdout, f)
	} else {
		h = log15.StreamHandler(os.Stderr, f)
	}

	if options.CallFunc {
		h = log15.CallerFuncHandler(h)
	}
	if options.CallStack {
		h = log15.CallerStackHandler("%+v", h)
	}

	switch options.Level {
	case 0:
		h = log15.DiscardHandler() // no output
	case 1:
		h = log15.LvlFilterHandler(log15.LvlCrit, h)
	case 2:
		h = log15.LvlFilterHandler(log15.LvlError, h)
	case 3:
		h = log15.LvlFilterHandler(log15.LvlWarn, h)
	case 4:
		h = log15.LvlFilterHandler(log15.LvlInfo, h)
	case 5:
		h = log15.LvlFilterHandler(log15.LvlDebug,
			verbosityFilterHandler(V(options.DebugV),
				matchAnyFilterHandler(options.DebugMatchKeyValuePairs, h, !options.DebugMatchExclude)))
	default:
		h = log15.LvlFilterHandler(log15.LvlInfo, h)
	}
	log15.Root().SetHandler(h)

	// Necessary to stop glog from complaining / noisy logs
	flag.CommandLine.Parse([]string{})
}

func verbosityFilterHandler(l V, h log15.Handler) log15.Handler {
	return log15.FilterHandler(func(r *log15.Record) bool {
		if l == 0 {
			return true
		}
		for i := 0; i < len(r.Ctx); i += 2 {
			switch v := r.Ctx[i+1].(type) {
			case V:
				return !(v > l)
			default:
				continue
			}
		}
		return true
	}, h)
}

func matchAnyFilterHandler(kv []string, h log15.Handler, log bool) log15.Handler {
	index := map[interface{}]map[interface{}]struct{}{}
	for _, s := range kv {
		p := strings.Split(s, "=")
		if len(p) != 2 {
			fmt.Fprintf(os.Stderr, "Bad filter: %v\n", s)
			continue
		}

		if _, has := index[p[0]]; !has {
			index[p[0]] = map[interface{}]struct{}{}
		}
		index[p[0]][p[1]] = struct{}{}
	}

	return log15.FilterHandler(func(r *log15.Record) bool {
		if len(index) == 0 {
			return true
		}

		// find ANY of the context keys and values in the index
		for i := 0; i < len(r.Ctx); i += 2 {
			if m, has := index[r.Ctx[i]]; has {
				check := fmt.Sprintf("%v", r.Ctx[i+1])
				if _, has := m[check]; has {
					return log
				}
			}
		}
		return !log
	}, h)
}
