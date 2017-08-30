package instance

import (
	"fmt"
	"strings"
)

func mergeLabelsIntoTagSlice(tags []interface{}, labels map[string]string) []string {
	m := map[string]string{}
	for _, l := range tags {
		line := fmt.Sprintf("%v", l) // conversion using string
		if i := strings.Index(line, ":"); i > 0 {
			key := line[0:i]
			value := ""
			if i+1 < len(line) {
				value = line[i+1:]
			}
			m[key] = value
		} else {
			m[fmt.Sprintf("%v", l)] = ""
		}
	}
	for k, v := range labels {
		m[k] = v
	}

	// now set the final format
	lines := []string{}
	for k, v := range m {
		if v != "" {
			lines = append(lines, fmt.Sprintf("%v:%v", k, v))
		} else {
			lines = append(lines, k)

		}
	}
	return lines
}
