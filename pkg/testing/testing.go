package testing

import (
	"fmt"
	"os"
	"strings"
)

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
