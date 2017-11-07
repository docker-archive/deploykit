package utils

import "strings"

// Sanatize ...
func Sanatize(s string) string {
	return strings.TrimRight(s, "/")
}

// IsEmpty ...
// see http://golang.org/ref/spec#Assignability
func IsEmpty(s string) bool {
	if s == "" || len(strings.TrimSpace(s)) == 0 {
		return true
	}
	return false
}
