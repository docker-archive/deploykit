package plugin

import (
	"strings"
)

// GetLookupAndType returns the plugin name for lookup and sub-type supported by the plugin.
// The name follows a microformat of $plugin[/$subtype] where $plugin is used for the discovery / lookup by name.
// The $subtype is used for the Type parameter in the RPC requests.
// Example: instance-file/json means lookup socket file 'instance-file' and the type is 'json'.
func GetLookupAndType(name string) (string, string) {
	if first := strings.Index(name, "/"); first >= 0 {
		return name[0:first], name[first+1:]
	}
	return name, ""
}
