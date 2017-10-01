package instance

import (
	"math/rand"
	"sort"
)

func asTagMap(m *map[string]*string) map[string]string {
	x := map[string]string{}
	if m == nil {
		return x
	}

	for k, v := range *m {
		if v != nil {
			x[k] = *v
		} else {
			x[k] = ""
		}
	}
	return x
}

func formatTags(m map[string]string) *map[string]*string {
	x := map[string]*string{}

	for k, v := range m {
		copy := v
		x[k] = &copy
	}
	return &x
}

// mergeTags merges multiple maps of tags, implementing 'last write wins' for colliding keys.
// Returns a sorted slice of all keys, and the map of merged tags.  Sorted keys are particularly useful to assist in
// preparing predictable output such as for tests.
func mergeTags(tagMaps ...map[string]string) ([]string, map[string]string) {
	keys := []string{}
	tags := map[string]string{}
	for _, tagMap := range tagMaps {
		for k, v := range tagMap {
			if _, exists := tags[k]; exists {
				log.Warn("Overwriting tag value", "key", k)
			} else {
				keys = append(keys, k)
			}
			tags[k] = v
		}
	}
	sort.Strings(keys)
	return keys, tags
}

func hasDifferentTags(expected, actual map[string]string) bool {
	if len(actual) == 0 {
		return true
	}
	for k, v := range expected {
		if a, ok := actual[k]; ok && a != v {
			return true
		}
	}

	return false
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

// RandomSuffix generate a random instance name suffix of length `n`.
func randomSuffix(n int) string {
	suffix := make([]rune, n)

	for i := range suffix {
		suffix[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	return string(suffix)
}
