package sshutil

import (
	"golang.org/x/crypto/ssh"
)

// CommandRunner executes shell commands on remote hosts.
type CommandRunner interface {
	Exec(addr string, config *ssh.ClientConfig, command string) (*string, error)
}

// NewCommandRunner returns a new CommandRunner that connects via TCP.
func NewCommandRunner() CommandRunner {
	return &remoteCommandRunner{}
}

type remoteCommandRunner struct {
}

// Exec executes a command on a remote host via SSH.
func (r remoteCommandRunner) Exec(addr string, config *ssh.ClientConfig, command string) (*string, error) {
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	outputString := string(output)
	return &outputString, err
}
