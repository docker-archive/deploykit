package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDuration(t *testing.T) {

	d := FromDuration(5 * time.Second)
	require.Equal(t, 5*time.Second, d.Duration())

	require.Equal(t, 2*time.Second, d.AtMost(2*time.Second))
	require.Equal(t, 5*time.Second, d.AtMost(10*time.Second))
	require.Equal(t, 10*time.Second, d.AtLeast(10*time.Second))
	require.Equal(t, 5*time.Second, d.AtLeast(1*time.Second))

	d2 := FromDuration(1 * time.Minute)
	require.Equal(t, 1*time.Minute, d2.Duration())

	list := []time.Duration{d2.Duration(), d.Duration()}
	SortDurations(list)

	require.Equal(t, Durations{d.Duration(), d2.Duration()}, Durations(list))

	test := struct {
		Poll Duration
	}{
		Poll: Duration(5 * time.Second),
	}

	require.NoError(t, AnyString(`{"Poll":"5m"}`).Decode(&test))
	require.Equal(t, 5*time.Minute, test.Poll.Duration())

	require.Equal(t, "{\n\"Poll\": \"5m0s\"\n}", mustString(AnyValueMust(test).MarshalJSON()))
}

func mustString(b []byte, err error) string {
	if err != nil {
		panic(err)
	}
	return string(b)
}
