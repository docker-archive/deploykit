package types

import (
	"strconv"
)

// MustParseUint parses a string into an uint.  Panics if not correct format
func MustParseUint(s string) uint {
	v, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return uint(v)
}
