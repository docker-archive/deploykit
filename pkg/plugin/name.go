package plugin

import (
	"strings"
)

// Name is a reference to the plugin.  Places where it appears include JSON files as type of field `Plugin`.
type Name string

// GetLookupAndType returns the plugin name for lookup and sub-type supported by the plugin.
// The name follows a microformat of $plugin[/$subtype] where $plugin is used for the discovery / lookup by name.
// The $subtype is used for the Type parameter in the RPC requests.
// Example: instance-file/json means lookup socket file 'instance-file' and the type is 'json'.
func (r Name) GetLookupAndType() (string, string) {
	name := string(r)
	if first := strings.Index(name, "/"); first >= 0 {
		return name[0:first], name[first+1:]
	}
	return name, ""
}

// String returns the string representation
func (r Name) String() string {
	return string(r)
}
