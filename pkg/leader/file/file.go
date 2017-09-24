package file

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/docker/infrakit/pkg/leader"
	logutil "github.com/docker/infrakit/pkg/log"
)

var (
	log    = logutil.New("module", "leader/file")
	debugV = logutil.V(1000)
)

// NewDetector return an implementation of leader detector
// This implementation checks a file for its content.  If the content matches the id of the detector
// then this instance is the leader.
func NewDetector(pollInterval time.Duration, filename, id string) (*leader.Poller, error) {
	// file must exist
	info, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, fmt.Errorf("file %s must be a file", filename)
	}

	return leader.NewPoller(pollInterval,
		func() (bool, error) {
			content, err := ioutil.ReadFile(filename)

			match := strings.Trim(string(content), " \t\n")

			log.Debug("poll for leadership", "id", id, "file", filename, "match", match, "err", err, "V", debugV)

			return match == id, err
		}), nil
}

// Store is the location of a file that stores the location of the leader
type Store string

// NewStore returns the store implementation
func NewStore(s string) leader.Store {
	return Store(s)
}

// UpdateLocation writes the location to the file.
func (s Store) UpdateLocation(location *url.URL) error {
	return ioutil.WriteFile(string(s), []byte(location.String()), 0644)
}

// GetLocation returns the location of the leader
func (s Store) GetLocation() (*url.URL, error) {
	content, err := ioutil.ReadFile(string(s))
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, nil
	}

	return url.Parse(string(content))
}
