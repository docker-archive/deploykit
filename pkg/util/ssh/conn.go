package ssh

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/util/exec"
	"golang.org/x/crypto/ssh"
)

var (
	log    = logutil.New("module", "util/ssh")
	debugV = logutil.V(300)
)

// Conn wraps an ssh client
type Conn struct {
	client *ssh.Client

	// Remote is the remote host:port
	Remote HostPort

	// Config is the SSH configuration.  There's a default one that uses
	// SSH agent
	Config *ssh.ClientConfig
}

// connect establishes a new, persistent SSH connection.
func (c *Conn) connect() error {
	log.Debug("connect", "conn", c, "V", debugV)

	sshAgent := false
	if c.Config == nil {
		config := DefaultClientConfig()
		c.Config = &config
		sshAgent = true
	}

	client, err := ssh.Dial("tcp", string(c.Remote), c.Config)
	if err != nil {
		log.Error("cannot connect", "host", c.Remote, "agent", sshAgent, "err", err)
		return err
	}

	c.client = client
	return nil
}

// Exec returns the exec.Interface implemented by the ssh session
func (c *Conn) Exec() (exec.Interface, error) {
	s, err := c.newSession()
	if err != nil {
		return nil, err
	}
	return &execImpl{Session: s}, nil
}

func (c *Conn) newSession() (*ssh.Session, error) {
	if c.client != nil {
		if s, err := c.client.NewSession(); err == nil {
			return s, err
		} else if err == io.EOF {
			// EOF means c.client is no longer connected.
			c.client = nil
		}
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c.client.NewSession()
}

// WriteFile writes data from an io.Reader to a file on the machine with perm.
func (c *Conn) WriteFile(filePath string, data io.Reader, perm os.FileMode) error {

	// TODO - replace this with sftp implementation

	log.Debug("write file", "path", filePath, "perm", perm, "V", debugV)

	bytes, err := ioutil.ReadAll(data)
	if err != nil {
		return err
	}

	s, err := c.newSession()
	if err != nil {
		return err
	}
	defer func(session *ssh.Session) {
		if err := session.Close(); err != nil {
			if err != io.EOF {
				log.Error("error", "err", err)
			}
		}
	}(s)

	stdin, err := s.StdinPipe()
	if err != nil {
		return err
	}

	if err := s.Start("scp -t -- " + filePath); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(stdin, "C%04o %d %s\n", perm, len(bytes), path.Base(filePath)); err != nil {
		return err
	}
	if _, err := stdin.Write(bytes); err != nil {
		return err
	}
	if _, err := stdin.Write([]byte{0}); err != nil {
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}

	return s.Wait()
}
