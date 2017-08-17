package file

import (
	"io/ioutil"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/leader"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	dir := os.TempDir()
	file, err := ioutil.TempFile(dir, "infrakit-file-test-location")
	require.NoError(t, err)

	store := Store(file.Name())

	loc := "tcp://10.10.1.100:24864"
	u, err := url.Parse(loc)
	require.NoError(t, err)

	err = store.UpdateLocation(u)
	require.NoError(t, err)

	u2, err := store.GetLocation()
	require.NoError(t, err)
	require.Equal(t, u, u2)
}

func TestFileDetector(t *testing.T) {

	dir := os.TempDir()
	file, err := ioutil.TempFile(dir, "infrakit-file-test")
	require.NoError(t, err)

	err = ioutil.WriteFile(file.Name(), []byte("instance1"), 0644)
	require.NoError(t, err)

	detector1, err := NewDetector(10*time.Millisecond, file.Name(), "instance1")
	require.NoError(t, err)
	detector2, err := NewDetector(10*time.Millisecond, file.Name(), "instance2")
	require.NoError(t, err)

	events1, err1 := detector1.Start()
	require.NoError(t, err1)
	require.NotNil(t, events1)

	events2, err2 := detector2.Start()
	require.NoError(t, err2)
	require.NotNil(t, events2)

	instance1 := make(chan bool)
	instance2 := make(chan bool)

	go func() {
		for event := range events1 {
			if event.Status == leader.Leader {
				instance1 <- true
			}
		}
	}()

	go func() {
		for event := range events2 {
			if event.Status == leader.Leader {
				instance2 <- true
			}
		}
	}()

	count := 5
	leader := []string{}
loop:
	for {
		select {

		case <-instance1:

			// It's instance1 for a while and then we switch to instance2
			count += -1
			if count == 0 {
				err = ioutil.WriteFile(file.Name(), []byte("instance2"), 0644)
				require.NoError(t, err)
				count = 5
				leader = append(leader, "instance1")
			}

		case <-instance2:

			count += -1
			if count == 0 {
				leader = append(leader, "instance2")
				break loop
			}
		}
	}

	detector1.Stop()
	detector2.Stop()

	require.Equal(t, []string{"instance1", "instance2"}, leader)
}
