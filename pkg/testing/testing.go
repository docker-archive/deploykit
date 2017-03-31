package testing

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
)

func init() {
	flag.Parse()
	flag.CommandLine.Parse([]string{"--logtostderr", "--v", "100"})
}

// TMustNoError panics if any arg is an error
func TMustNoError(args ...interface{}) []interface{} {
	for _, v := range args {
		if _, is := v.(error); is {
			panic(v)
		}
	}
	return ([]interface{})(args)
}

// SkipTests returns true if the environment SKIP_TESTS has the input value in its comma-delimited list
func SkipTests(check string) bool {
	list := strings.Split(os.Getenv("SKIP_TESTS"), ",")
	for _, v := range list {
		if v == check {
			fmt.Println("Env SKIP_TESTS has", check, "-- skipping.")
			return true
		}
	}
	fmt.Println("Env SKIP_TESTS does not have", check, "-- continue.")
	return false
}

// T returns a verbose logger with line output
func T(level int) glog.Verbose {
	return glog.V(glog.Level(level))
}
