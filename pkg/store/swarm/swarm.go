package swarm

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/swarm"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/util/docker"
	"golang.org/x/net/context"
)

var (
	log = logutil.New("module", "store/swarm")
)

type snapshot struct {
	client docker.APIClientCloser
	key    string
}

// NewSnapshot returns an instance of the snapshot service where data is stored as a label
// in the swarm raft store.
func NewSnapshot(client docker.APIClientCloser, key string) (store.Snapshot, error) {
	return &snapshot{client: client, key: key}, nil
}

// Save saves a snapshot of the given object and revision.
func (s *snapshot) Save(obj interface{}) error {
	label, err := encode(obj)
	if err != nil {
		return err
	}
	return writeSwarm(s.client, s.key, label)
}

// Load loads a snapshot and marshals into the given reference
func (s *snapshot) Load(output interface{}) error {
	label, err := readSwarm(s.client, s.key)
	if err == nil {
		return decode(label, output)
	}
	if err != errNotFound {
		return err
	}
	return nil
}

// Close implements io.Closer
func (s *snapshot) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}

var errNotFound = fmt.Errorf("not-found")

func readSwarm(client docker.APIClientCloser, key string) (string, error) {
	info, err := client.SwarmInspect(context.Background())
	if err != nil {
		return "", err
	}

	if info.ClusterInfo.Spec.Annotations.Labels != nil {
		if l, has := info.ClusterInfo.Spec.Annotations.Labels[key]; has {
			log.Debug("readSwarm", "config", l)
			return l, nil
		}
	}
	return "", errNotFound
}

func writeSwarm(client docker.APIClientCloser, key, value string) error {
	attempt := 0
	maxAttempts := 10
	for {
		info, err := client.SwarmInspect(context.Background())
		if err != nil {
			log.Error("Failed to inspect swarm", "err", err)
			return err
		}
		if info.ClusterInfo.Spec.Annotations.Labels == nil {
			info.ClusterInfo.Spec.Annotations.Labels = map[string]string{}
		}
		info.ClusterInfo.Spec.Annotations.Labels[key] = value
		log.Debug("Updating swarm data",
			"version", info.ClusterInfo.Meta.Version.Index,
			"attempt", attempt)
		err = client.SwarmUpdate(
			context.Background(),
			info.ClusterInfo.Meta.Version,
			info.ClusterInfo.Spec,
			swarm.UpdateFlags{})
		if err == nil {
			break
		}
		attempt++
		// Sleep and retry when the version is out of sequence
		if attempt < maxAttempts && strings.Contains(err.Error(), "update out of sequence") {
			log.Info("Unable to update swarm data due to version conflict, will retry",
				"version", info.ClusterInfo.Meta.Version.Index,
				"attempt", attempt,
				"err", err)
			time.Sleep(time.Second * time.Duration(attempt))
			continue
		}
		log.Error("Failed to update swarm data", "err", err)
		return err
	}
	return nil
}

func encode(obj interface{}) (string, error) {
	buff, err := json.MarshalIndent(obj, "  ", "  ")
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write(buff)
	w.Close()
	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

func decode(label string, output interface{}) error {
	data, err := base64.StdEncoding.DecodeString(label)
	if err != nil {
		return err
	}
	b := bytes.NewBuffer(data)
	r, err := zlib.NewReader(b)

	var inflate bytes.Buffer
	io.Copy(&inflate, r)
	r.Close()

	return json.Unmarshal(inflate.Bytes(), output)
}
