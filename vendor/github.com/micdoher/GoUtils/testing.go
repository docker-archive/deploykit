package utils

import "testing"

func FailOnError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
