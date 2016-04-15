package aws

import (
	"time"
)

func makePointerSlice(stackSlice []string) []*string {
	pointerSlice := []*string{}
	for i := range stackSlice {
		pointerSlice = append(pointerSlice, &stackSlice[i])
	}
	return pointerSlice
}

// WaitUntil executes the given function up to the given attempts, with given duration as wait time
// between executions.  It returns before the max attempts if the function returns true or has an
// error.
func WaitUntil(
	sleeper func(time.Duration),
	maxAttempts int,
	waitInterval time.Duration,
	f func() (bool, error)) error {

	for i := 0; i < maxAttempts; i++ {
		stop, err := f()
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
		sleeper(waitInterval)
	}
	return &ErrExceededAttempts{maxAttempts}
}
