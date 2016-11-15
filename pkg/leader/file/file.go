package file

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/leader"
)

// NewDetector return an implementation of leader detector
// This implementation checks a file for its content.  If the content matches the id of the detector
// then this instance is the leader.
func NewDetector(pollInterval time.Duration, filename, id string) (leader.Detector, error) {
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

			log.Debugf("ID (%s) - checked %s for leadership: %s, err=%v, leader=%v", id, filename, match, err, match == id)

			return match == id, err
		}), nil
}
