package file

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/leader"
)

const (
	// LeaderFileEnvVar is the environment variable that may be used to customize the plugin leader detection
	LeaderFileEnvVar = "INFRAKIT_LEADER_FILE"
)

// DefaultLeaderFile is the file that this detector uses to decide who the leader is.
// In a mult-host set up, it's assumed that the file system would be share (e.g. NFS mount or S3 FUSE etc.)
func DefaultLeaderFile() string {
	if leaderFile := os.Getenv(LeaderFileEnvVar); leaderFile != "" {
		return leaderFile
	}

	home := os.Getenv("HOME")
	if usr, err := user.Current(); err == nil {
		home = usr.HomeDir
	}
	return filepath.Join(home, ".infrakit/leader")
}

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
