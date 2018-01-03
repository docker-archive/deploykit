package ssh

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// HostPort is host:port
type HostPort string

// Tunnel is a forwarder of local ssh traffic to a remote endpoint
type Tunnel struct {
	// Local is the local endpoint for all clients
	Local HostPort
	// Server is the middle server listening on SSH port
	Server HostPort
	// Remote is the backend that's not accessible without going through the server endpoint
	Remote HostPort

	Config *ssh.ClientConfig
}

// RandPort picks a port from the given range randomly.  It doesn't check if a port has been allocated.
func RandPort(lo, hi int) int {
	return lo + rand.Intn(hi-lo)
}

// DefaultClientConfig returns the default settings of the ssh client
func DefaultClientConfig() ssh.ClientConfig {
	return ssh.ClientConfig{
		Auth: []ssh.AuthMethod{
			Agent(),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO - add ways to set host's public key
	}
}

// Start starts the tunnel
func (tunnel *Tunnel) Start() error {
	return <-tunnel.startAsync()
}

func (tunnel *Tunnel) startAsync() <-chan error {
	ready := make(chan error, 1)
	go func() {
		listener, err := net.Listen("tcp", string(tunnel.Local))
		if err != nil {
			ready <- err
			close(ready)
			return
		}
		defer listener.Close()
		close(ready) // notify caller before we block
		for {
			conn, err := listener.Accept()
			if err != nil {
				panic(err)
			}
			go tunnel.forward(conn)
		}
	}()
	return ready
}

func (tunnel *Tunnel) forward(localConn net.Conn) {
	serverConn, err := ssh.Dial("tcp", string(tunnel.Server), tunnel.Config)
	if err != nil {
		fmt.Printf("Server dial error: %s\n", err)
		localConn.Close()
		return
	}

	remoteConn, err := serverConn.Dial("tcp", string(tunnel.Remote))
	if err != nil {
		fmt.Printf("Remote dial error: %s\n", err)
		localConn.Close()
		return
	}

	copyConn := func(writer, reader net.Conn) {
		_, err := io.Copy(writer, reader)
		if err != nil {
			fmt.Printf("io.Copy error: %s", err)
		}
	}

	go copyConn(localConn, remoteConn)
	go copyConn(remoteConn, localConn)
}
