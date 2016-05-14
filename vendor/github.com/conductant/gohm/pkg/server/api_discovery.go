package server

import (
	"fmt"
	"golang.org/x/net/context"
	"net/http"
	"reflect"
	"regexp"
	"runtime"
	"strings"
)

var funcNameRegexp = regexp.MustCompile(
	"(?P<package>[a-zA-Z0-9\\./]+)\\.(?P<var>\\([a-zA-Z0-9\\*]+\\))\\.\\(*(?P<method>[a-zA-Z0-9\\./]+)\\)*")

// For sanitizing the various ways in which the compiler encodes the function name into a single consistent way.
// (package_name).(struct_name).(package_name).(struct_name).(function_name).
// Example: github.com/conductant/gohm/pkg/server.(*testE2EServer).(github.com/conductant/gohm/pkg/server.testSimpleGet)-fm
//          github.com/conductant/gohm/pkg/server.(*testE2EServer).(github.com/conductant/gohm/pkg/server.testGetApiFromContext)-fm
//          github.com/conductant/gohm/pkg/server.(*testE2EServer).testGetApiFromContext
func cleanFuncName(f string) string {
	fn := strings.Split(f, "-")[0]

	if funcNameRegexp.MatchString(f) {
		p := funcNameRegexp.FindStringSubmatch(f)
		pp := strings.Split(p[3], ".")
		method := pp[0]
		if len(pp) > 1 {
			// sometimes the method name is fully qualified.  Strip and take the last part
			method = pp[len(pp)-1]
		}
		fn = fmt.Sprintf("%s.%s", p[1], method)
	} else {
		fn = strings.Replace(fn, "(", "", -1)
		fn = strings.Replace(fn, ")", "", -1)
		fn = strings.Replace(fn, "*", "", -1)
	}
	return fn
}

func (this *engine) ApiForScope() Endpoint {
	if pc, _, _, ok := runtime.Caller(1); ok {
		return this.apiFromPC(pc)
	}
	return Endpoint{}
}

func (this *engine) ApiForFunc(f func(context.Context, http.ResponseWriter, *http.Request)) Endpoint {
	pc := reflect.ValueOf(f).Pointer()
	return this.apiFromPC(pc)
}

func (this *engine) apiFromPC(pc uintptr) Endpoint {
	callingFunc := cleanFuncName(runtime.FuncForPC(pc).Name())
	if binding, exists := this.functionNames[callingFunc]; exists {
		return binding.Api
	}
	return Endpoint{}

}
