package azure

import (
	"github.com/docker/editions/pkg/loadbalancer"
	"github.com/stretchr/testify/require"
	"testing"
)

type testResponse string

func (t testResponse) String() string {
	return string(t)
}

// TestRunAsyncForDuration tests the async handling of calling azure autorest long running api calls.
func TestRunAsynForDuration(t *testing.T) {

	task := func(cancel <-chan struct{}) (loadbalancer.Result, error) {
		require.NotNil(t, cancel)

		<-cancel
		return testResponse("done"), nil
	}
	// Test non-blocking call
	resp, err := runAsyncForDuration(100, task) // takes up to 100 seconds -- failure hangs.
	require.NoError(t, err)
	require.Equal(t, (asyncResponse{}).String(), resp.String())
}

// TestWaitFor blocks until result is back
func TestWaitFor(t *testing.T) {

	task := func(cancel <-chan struct{}) (loadbalancer.Result, error) {
		require.NotNil(t, cancel)

		<-cancel
		return testResponse("done"), nil
	}

	// Test for blocking wait
	result, err := WaitFor(runAsyncForDuration(1, task))
	require.NoError(t, err)
	require.Equal(t, "done", result.String())
}
