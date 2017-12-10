package ssh

import (
	"io"
	"strings"

	"golang.org/x/crypto/ssh"
)

type execImpl struct {
	cmd []string
	*ssh.Session
}

type readcloser struct {
	io.Reader
}

func (r readcloser) Close() error {
	return nil
}

func (s *execImpl) StdoutPipe() (io.ReadCloser, error) {
	r, err := s.Session.StdoutPipe()
	return readcloser{r}, err
}

func (s *execImpl) StderrPipe() (io.ReadCloser, error) {
	r, err := s.Session.StderrPipe()
	return readcloser{r}, err
}

// SetCmd implements exec.Interface; sets the command string
func (s *execImpl) SetCmd(v []string) {
	s.cmd = v
}

// SetEnv implements exec.Interface; sets the environment variables before execution
func (s *execImpl) SetEnv(v []string) {
	for _, p := range v {
		kv := strings.SplitN(p, "=", 2)
		err := s.Setenv(kv[0], kv[1])
		if err != nil {
			log.Error("cannot set environment variable", "v", kv[0])
		}
	}
}

// SetDir implements exec.Interface; not supported
func (s *execImpl) SetDir(v string) {
	log.Warn("setDir not supported", "dir", v)
	// not supported
	return
}

// SetStdout implements exec.Interface
func (s *execImpl) SetStdout(v io.Writer) {
	stdout, err := s.StdoutPipe()
	if err != nil {
		log.Error("set stdout", "err", err)
		return
	}
	go io.Copy(v, stdout)
}

// SetStderr implements exec.Interface
func (s *execImpl) SetStderr(v io.Writer) {
	stderr, err := s.StderrPipe()
	if err != nil {
		log.Error("set stderr", "err", err)
		return
	}
	go io.Copy(v, stderr)
}

// SetStdin implements exec.Interface
func (s *execImpl) SetStdin(v io.Reader) {
	stdin, err := s.StdinPipe()
	if err != nil {
		log.Error("set stdin", "err", err)
		return
	}
	defer stdin.Close()
	go io.Copy(stdin, v)
}

// Start implements exec.Interface
func (s *execImpl) Start() error {
	return s.Session.Start(strings.Join(s.cmd, " "))
}

// Wait implements exec.Interface
func (s *execImpl) Wait() error {
	return s.Session.Wait()
}

// Output implements exec.Interface
func (s *execImpl) Output() ([]byte, error) {
	return s.Session.Output(strings.Join(s.cmd, " "))
}
