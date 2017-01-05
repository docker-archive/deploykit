package template

import (
	"os"
)

// GetDefaultContextURL returns the default context URL if none is known.
func GetDefaultContextURL() string {
	pwd := "/"
	if wd, err := os.Getwd(); err == nil {
		pwd = wd
	} else {
		pwd = os.Getenv("PWD")
	}
	return "file://localhost" + pwd + "/"
}
