package etcd

import (
	"net"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/docker/infrakit/pkg/util/exec"
)

// Options is for configuring the snapshot client
type Options struct {
	clientv3.Config

	// RequestTimeout is used for all requests to etcd
	RequestTimeout time.Duration
}

// Client is a wrapper for etcd
type Client struct {
	Client  *clientv3.Client
	Options Options
}

// NewClient returns a client
func NewClient(options Options) (*Client, error) {
	cli, err := clientv3.New(options.Config)
	if err != nil {
		return nil, err
	}
	return &Client{
		Client:  cli,
		Options: options,
	}, nil
}

// Close closes the connection
func (c *Client) Close() error {
	if c.Client != nil {
		return c.Client.Close()
	}
	return nil
}

// LocalIP returns the first non loopback local IP of the host
func LocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

var (

	// RunContainer is a command that shells out to Docker to run the etcd server in a container
	RunContainer = exec.Command(`
docker run --rm -d \
       -v /usr/share/ca-certificates/:/etc/ssl/certs \
       -p 4001:4001 \
       -p 2380:2380 \
       -p 2379:2379 \
       --name {{ arg 2 }} \
       quay.io/coreos/etcd etcd \
       -debug \
       -name etcd0 \
       -advertise-client-urls http://{{ arg 1 }}:2379,http://{{ arg 1 }}:4001 \
       -listen-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001 \
       -initial-advertise-peer-urls http://{{ arg 1 }}:2380 \
       -listen-peer-urls http://0.0.0.0:2380 \
       -initial-cluster-token etcd-cluster-1 \
       -initial-cluster etcd0=http://{{ arg 1 }}:2380 \
       -initial-cluster-state new
`)

	// StopContainer stops the etcd container
	StopContainer = exec.Command(`docker stop {{ arg 1 }}`)

	// LsMembers lists the members in the cluster
	LsMembers = exec.Command(`
docker run --rm -e ETCDCTL_API=3 \
       quay.io/coreos/etcd etcdctl --endpoints={{ arg 1 }}:2379 member list
`)

	// Get fetches a value via etcdctl
	Get = exec.Command(`
docker run --rm -e ETCDCTL_API=3 \
       quay.io/coreos/etcd etcdctl --endpoints={{ arg 1 }}:2379 get --print-value-only {{ arg 2 }}
`)
)
