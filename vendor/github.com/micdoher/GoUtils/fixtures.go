package utils

import (
	"io/ioutil"
)

// This method implies that from the context where the test is being run
// a "fixtures" folder exists.
func Fixture(fn string) ([]byte, error) {
	return ioutil.ReadFile("./fixtures/" + fn)
}
