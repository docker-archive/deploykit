package plugin

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Names is a list of Names
type Names []Name

// NamesFrom returns a slice of Names from a string list
func NamesFrom(list []string) Names {
	n := []Name{}
	for _, v := range list {
		n = append(n, Name(v))
	}
	return Names(n)
}

// Name is a reference to the plugin.  Places where it appears include JSON files as type of field `Plugin`.
type Name string

// Zero returns true if the name is zero value
func (n Name) Zero() bool {
	return len(n) == 0
}

// NameFrom creates a name from the parts
func NameFrom(lookup, sub string) Name {
	if sub != "" {
		return Name(strings.Join([]string{lookup, sub}, "/"))
	}
	return Name(lookup)
}

// Rel computes the relative form, assuming the receiver is the base path
// so that Join(other, n.Rel(other)) == other.  If it's not relative at all, then the receiver is returned
func (n Name) Rel(other Name) Name {
	rel, err := filepath.Rel(other.String(), n.String())
	if err == nil {
		if rel == "." {
			return n
		}
		return Name(rel)
	}
	return n
}

// Lookup returns the lookup portion of the name
func (n Name) Lookup() string {
	lookup, _ := n.GetLookupAndType()
	return lookup
}

// LookupOnly returns the trailing slash form e.g. us-east/
func (n Name) LookupOnly() Name {
	lookup, _ := n.GetLookupAndType()
	return Name(string(lookup + "/"))
}

// WithType returns a new name with the same lookup but a different type
func (n Name) WithType(t interface{}) Name {
	return Name(fmt.Sprintf("%v/%v", n.Lookup(), t))
}

// Sub is the same as WithType
func (n Name) Sub(v string) Name {
	return n.WithType(v)
}

// Equal returns true if the other name is the same
func (n Name) Equal(other Name) bool {
	return string(n) == string(other)
}

// HasType returns true if the name is of the form lookup/type
func (n Name) HasType() bool {
	_, s := n.GetLookupAndType()
	return s != ""
}

// Type returns the second portion of the name 'ec2-instance' in 'aws/ec2-instance'
func (n Name) Type() string {
	_, t := n.GetLookupAndType()
	return t
}

// IsEmpty returns true if the name is an empty string
func (n Name) IsEmpty() bool {
	return string(n) == ""
}

// String returns the string form
func (n Name) String() string {
	return string(n)
}

// GetLookupAndType returns the plugin name for lookup and sub-type supported by the plugin.
// The name follows a microformat of $plugin[/$subtype] where $plugin is used for the discovery / lookup by name.
// The $subtype is used for the Type parameter in the RPC requests.
// Example: instance-file/json means lookup socket file 'instance-file' and the type is 'json'.
func (n Name) GetLookupAndType() (string, string) {
	name := string(n)
	if first := strings.Index(name, "/"); first >= 0 {
		return name[0:first], name[first+1:]
	}
	return name, ""
}

// MarshalJSON implements the JSON marshaler interface
func (n Name) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, n.String())), nil
}

// UnmarshalJSON implements the JSON unmarshaler interface
func (n *Name) UnmarshalJSON(data []byte) error {
	str := string(data)
	start := strings.Index(str, "\"")
	last := strings.LastIndex(str, "\"")
	if start == 0 && last == len(str)-1 {
		str = str[start+1 : last]
	} else {
		return fmt.Errorf("bad-format-for-name:%v", string(data))
	}
	*n = Name(str)
	return nil
}
