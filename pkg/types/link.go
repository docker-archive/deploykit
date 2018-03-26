package types

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const (
	letters = "abcdefghijklmnopqrstuvwxyz0123456789"
)

func init() {
	rand.Seed(int64(time.Now().Nanosecond()))
}

// Link is a struct that represents an association between an infrakit managed resource
// and an entity in some other system.  The mechanism of linkage is via labels or tags
// on both sides.
type Link struct {
	value   string
	context string
	created time.Time
}

// NewLink creates a link
func NewLink() *Link {
	return &Link{
		value:   randomAlphaNumericString(16),
		created: time.Now(),
	}
}

// Link related labels
const (
	LinkLabel        = "infrakit_link"
	LinkContextLabel = "infrakit_link_context"
	LinkCreatedLabel = "infrakit_link_created"
)

// NewLinkFromMap constructs a link from data in the map. The link will have missing data
// if the input does not contain the attribute labels.
func NewLinkFromMap(m map[string]string) *Link {
	l := &Link{}
	if v, has := m[LinkLabel]; has {
		l.value = v
	}

	if v, has := m[LinkContextLabel]; has {
		l.context = v
	}
	if v, has := m[LinkCreatedLabel]; has {
		// The RFC3339 format requires upper case values
		t, err := time.Parse(time.RFC3339, strings.ToUpper(v))
		if err == nil {
			l.created = t
		}
	}
	return l
}

// Valid returns true if the link value is set
func (l Link) Valid() bool {
	return l.value != ""
}

// Value returns the value of the link
func (l Link) Value() string {
	return l.value
}

// Created returns the creation time of the link
func (l Link) Created() time.Time {
	return l.created
}

// Label returns the label to look for the link
func (l Link) Label() string {
	return LinkLabel
}

// Context returns the context of the link
func (l Link) Context() string {
	return l.context
}

// WithContext sets a context for this link
func (l *Link) WithContext(s string) *Link {
	l.context = s
	return l
}

// KVPairs returns the link representation as a slice of Key=Value pairs
func (l *Link) KVPairs() []string {
	out := []string{}
	for k, v := range l.Map() {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}

// Map returns a representation that is easily converted to JSON or YAML
func (l *Link) Map() map[string]string {
	return map[string]string{
		LinkLabel:        l.value,
		LinkContextLabel: l.context,
		LinkCreatedLabel: l.created.Format(time.RFC3339),
	}
}

// WriteMap writes to the target map.  This will overwrite values of same key. Target cannot be nil.
func (l *Link) WriteMap(target map[string]string) (merged map[string]string) {
	merged = target
	if merged == nil {
		merged = map[string]string{}
	}
	for k, v := range target {
		merged[k] = v
	}
	for k, v := range l.Map() {
		if v == "" {
			continue
		}
		merged[k] = v
	}
	return
}

// InMap returns true if the link is contained in the map
func (l *Link) InMap(m map[string]string) bool {
	c, has := m[LinkContextLabel]
	if !has {
		return false
	}
	if c != l.context {
		return false
	}

	v, has := m[LinkLabel]
	if !has {
		return false
	}
	return v == l.value
}

// Equal returns true if the links are the same - same value and context
func (l *Link) Equal(other *Link) bool {
	return l.value == other.value && l.context == other.context
}

// randomAlphaNumericString generates a non-secure random alpha-numeric string of a given length.
func randomAlphaNumericString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
