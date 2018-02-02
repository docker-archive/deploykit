package backend

import (
	"context"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/run/scope"
)

// Parameters interface provides an interface for the backend to
// declare the required parameters.  These parameters can be mapped
// to CLI flags or in UI form fields.
type Parameters interface {
	SetParameter(name string, value interface{}) error

	StringSlice(name string, value []string, usage string) *[]string
	String(name string, value string, usage string) *string
	Float64(name string, value float64, usage string) *float64
	Int(name string, value int, usage string) *int
	Bool(name string, value bool, usage string) *bool
	IP(name string, value net.IP, usage string) *net.IP
	Duration(name string, value time.Duration, usage string) *time.Duration

	GetStringSlice(name string) ([]string, error)
	GetString(name string) (string, error)
	GetFloat64(name string) (float64, error)
	GetInt(name string) (int, error)
	GetBool(name string) (bool, error)
	GetIP(name string) (net.IP, error)
	GetDuration(name string) (time.Duration, error)
}

// ExecFunc is the function of that backend that actually does work.
type ExecFunc func(ctx context.Context, script string, params Parameters, args []string) error

// ContextKey is the key to
type ContextKey int

const (
	// OutputContextKey is the context key used to get the output writer
	OutputContextKey ContextKey = iota
)

// SetWriter sets the io.Writer to use for a backend.
func SetWriter(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, OutputContextKey, w)
}

// GetWriter returns the io.Writer for writing output if it's provided based on the context
func GetWriter(ctx context.Context) io.Writer {
	if v := ctx.Value(OutputContextKey); v != nil {
		if w, is := v.(io.Writer); is {
			return w
		}
	}
	return os.Stdout
}

// DefineParamsFunc is the function for the backend to register command line flags
type DefineParamsFunc func(Parameters)

// ExecFuncBuilder is the type of function exported / available to the scripting template
type ExecFuncBuilder func(scope scope.Scope, trial bool, opt ...interface{}) (ExecFunc, error)

var (
	backends = map[string]ExecFuncBuilder{}
	flags    = map[string]DefineParamsFunc{}
	lock     = sync.Mutex{}
)

// Register registers a named backend.  The function parameters will be matched
// in the =% %= tags of backend specification.
func Register(funcName string, backend ExecFuncBuilder, buildFlags DefineParamsFunc) {
	lock.Lock()
	defer lock.Unlock()
	backends[funcName] = backend
	flags[funcName] = buildFlags
}

// VisitBackends visits all the backends.  The visitor is a function that is given a view of
// a function name bound to a generator function
func VisitBackends(visitor func(funcName string, backend ExecFuncBuilder)) {
	lock.Lock()
	defer lock.Unlock()

	for funcName, backend := range backends {
		visitor(funcName, backend)
	}
}

// DefineSharedParameters visits all the backends and if the backend has certain parameter
// requirements, it is reflected here (eg. build global flags,etc).
func DefineSharedParameters(visitor func(string, DefineParamsFunc)) {
	lock.Lock()
	defer lock.Unlock()

	for funcName, f := range flags {
		visitor(funcName, f)
	}
}
