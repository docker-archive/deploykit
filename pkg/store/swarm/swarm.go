package swarm

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/infrakit/pkg/store"
	"golang.org/x/net/context"
)

const (
	// SwarmLabel is the label for the swarm annotation that stores a compressed version of the config.
	SwarmLabel = "infrakit"
)

type snapshot struct {
	client client.APIClient
}

// NewSnapshot returns an instance of the snapshot service where data is stored as a label
// in the swarm raft store.
func NewSnapshot(client client.APIClient) (store.Snapshot, error) {
	return &snapshot{client: client}, nil
}

// Save saves a snapshot of the given object and revision.
func (s *snapshot) Save(obj interface{}) error {
	label, err := encode(obj)
	if err != nil {
		return err
	}
	return writeSwarm(s.client, label)
}

// Load loads a snapshot and marshals into the given reference
func (s *snapshot) Load(output interface{}) error {
	label, err := readSwarm(s.client)
	if err == nil {
		return decode(label, output)
	}
	if err != errNotFound {
		return err
	}
	return nil
}

var errNotFound = fmt.Errorf("not-found")

func readSwarm(client client.APIClient) (string, error) {
	info, err := client.SwarmInspect(context.Background())
	if err != nil {
		return "", err
	}

	if info.ClusterInfo.Spec.Annotations.Labels != nil {
		if l, has := info.ClusterInfo.Spec.Annotations.Labels[SwarmLabel]; has {
			log.Debugln("config=", l)
			return l, nil
		}
	}
	return "", errNotFound
}

func writeSwarm(client client.APIClient, value string) error {
	info, err := client.SwarmInspect(context.Background())
	if err != nil {
		return err
	}
	if info.ClusterInfo.Spec.Annotations.Labels == nil {
		info.ClusterInfo.Spec.Annotations.Labels = map[string]string{}
	}
	info.ClusterInfo.Spec.Annotations.Labels[SwarmLabel] = value
	return client.SwarmUpdate(context.Background(), info.ClusterInfo.Meta.Version, info.ClusterInfo.Spec,
		swarm.UpdateFlags{})
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
