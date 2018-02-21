package types

import (
	"sort"
	"strings"
	"time"
)

// Duration is a wrapper for time.Duration that adds JSON parsing
type Duration time.Duration

// Duration returns the time.Duration
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// AtLeast returns the a duration that is either greater or at least as long as the input
func (d Duration) AtLeast(min time.Duration) time.Duration {
	this := d.Duration()
	if min > this {
		return min
	}
	return this
}

// AtMost returns the a duration that is either less than or at most as long as the input
func (d Duration) AtMost(max time.Duration) time.Duration {
	this := d.Duration()
	if max < this {
		return max
	}
	return this
}

// FromDuration constructs a Duration from a time.Duration
func FromDuration(d time.Duration) Duration {
	return Duration(d)
}

// MustParseDuration returns a duration from a string, panics if can't parse
func MustParseDuration(s string) Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return Duration(d)
}

// Durations is a wrapper to make Duration sortable
type Durations []time.Duration

func (d Durations) Len() int           { return len(d) }
func (d Durations) Less(i, j int) bool { return time.Duration(d[i]) < time.Duration(d[j]) }
func (d Durations) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }

// SortDurations sorts the durations
func SortDurations(d []time.Duration) {
	sort.Sort(Durations(d))
}

// DurationFromString returns either the parsed duration or a default value.
func DurationFromString(s string, defaultValue time.Duration) Duration {
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return Duration(defaultValue)
	}
	return Duration(parsed)
}

// MarshalJSON returns the json representation
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte("\"" + time.Duration(d).String() + "\""), nil
}

// UnmarshalJSON unmarshals the buffer to this struct
func (d *Duration) UnmarshalJSON(buff []byte) error {
	parsed, err := time.ParseDuration(strings.Trim(string(buff), "\""))
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}
