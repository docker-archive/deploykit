package options

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/docker/infrakit/pkg/types"
)

var fileRegex = regexp.MustCompile("^file://(/.*):(.*)$")

// ParseEnvs processes the data to create a key=value slice of strings
func ParseEnvs(data *types.Any) ([]string, error) {
	envs := []string{}
	if data == nil || len(data.Bytes()) == 0 {
		return envs, nil
	}
	err := json.Unmarshal(data.Bytes(), &envs)
	if err != nil {
		return envs, err
	}
	err = processValues(&envs)
	if err != nil {
		return []string{}, err
	}
	return envs, err
}

// processValues updates the key=value slice entries with any custom lookups; supported lookups are:
// - A value in the form of "key=file://<path>:<some-key>", this loads the file at "path", unmarshals it
//   into a JSON object and then uses the value associated with the <some-key>.
//   For example, a value of "key=file:///run/secrets/my-secret:some-key" does the following:
//   1. Loads the file at "/run/secrets/my-secret" as a JSON map and retrieves the "some-key" value
//   2. Updates the slice element to "key=<file-value-of-some-key>"
func processValues(vals *[]string) error {
	result := make([]string, len(*vals))
	for i, val := range *vals {
		if !strings.Contains(val, "=") {
			return fmt.Errorf("Env var is missing '=' character: %v", val)
		}
		result[i] = val
		split := strings.SplitN(val, "=", 2)
		matches := fileRegex.FindStringSubmatch(split[1])
		if len(matches) != 3 {
			continue
		}
		fp := matches[1]
		fileKey := matches[2]
		buff, err := ioutil.ReadFile(fp)
		if err != nil {
			return fmt.Errorf("Failed to read file %s: %s", fp, err)
		}
		// Parse the JSON file as a map
		data := make(map[string]string)
		err = json.Unmarshal(buff, &data)
		if err != nil {
			return fmt.Errorf("Failed to parse file %s: %s", fp, err)
		}
		// And get the value for the given key
		if newVal, has := data[fileKey]; has {
			result[i] = fmt.Sprintf("%v=%v", split[0], newVal)
		} else {
			return fmt.Errorf("File '%s' does not contain key '%s'", fp, fileKey)
		}
	}
	*vals = result
	return nil
}
