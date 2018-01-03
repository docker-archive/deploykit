package ssh

import (
	"io/ioutil"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// UsernamePasswordConfig returns the auth method based on the password
func UsernamePasswordConfig(username, password string) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
}

// PublicKeyConfig returns a ssh config that uses public key
func PublicKeyConfig(username, file string) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			PublicKeyFile(file),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
}

// PublicKeyFile returns an auth method based on SSH public / private key
// The file parameter is the path to the private key.
func PublicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

// AgentConfig returns a ssh config that uses ssh agent
func AgentConfig(username string) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			Agent(),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
}

// Agent returns the auth method using SSH agent
func Agent() ssh.AuthMethod {
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}
	return nil
}
