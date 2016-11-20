package os

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sync"
	"syscall"

	log "github.com/Sirupsen/logrus"
)

const (
	// LogDirEnvVar is the environment variable that may be used to customize the plugin logs location
	LogDirEnvVar = "INFRAKIT_LOG_DIR"
)

// DefaultLogDir is the directory for storing the log files from the plugins
func DefaultLogDir() string {
	if logDir := os.Getenv(LogDirEnvVar); logDir != "" {
		return logDir
	}

	home := os.Getenv("HOME")
	if usr, err := user.Current(); err == nil {
		home = usr.HomeDir
	}
	return filepath.Join(home, ".infrakit/logs")
}

// NewLauncher returns a Launcher that can install and start plugins.  The OS version is simple - it translates
// plugin names as command names and uses os.Exec
func NewLauncher(logDir string) (*Launcher, error) {
	return &Launcher{
		logDir:  logDir,
		plugins: map[plugin]state{},
	}, nil
}

type plugin string
type state struct {
	log  string
	wait <-chan error
}

type Launcher struct {
	logDir  string
	plugins map[plugin]state
	lock    sync.Mutex
}

// Launch implements Launcher.Launch.  Returns a signal channel to block on optionally.
// The channel is closed as soon as an error (or nil for success completion) is written.
// The command is run in the background / asynchronously.  The returned read channel
// stops blocking as soon as the command completes (which uses shell to run the real task in
// background).
func (l *Launcher) Launch(cmd string, args ...string) (<-chan error, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if s, has := l.plugins[plugin(cmd)]; has {
		return s.wait, nil
	}

	_, err := exec.LookPath(cmd)
	if err != nil {
		return nil, err
	}

	wait := make(chan error)
	s := state{
		log:  l.buildLogfile(cmd, args...),
		wait: wait,
	}

	l.plugins[plugin(cmd)] = s

	sh := l.buildCmd(s.log, cmd, args...)

	go func() {

		log.Infoln("OS launcher: Plugin", cmd, "starting", sh)
		cmd := getShell()

		// Set new pgid so the process doesn't exit when the starter exits.
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
			Pgid:    0,
		}

		defer close(wait)

		// pipe the command string to the shell
		stdin, err := cmd.StdinPipe()
		if err != nil {
			wait <- err
			return
		}

		err = cmd.Start()
		if err != nil {
			log.Warningln("OS launcher: Plugin", cmd, "failed to start:", err, "cmd=", sh)
			wait <- err
			return
		}
		stdin.Write([]byte(sh))
		stdin.Close()

		wait <- cmd.Wait()
		return
	}()

	return wait, nil
}
