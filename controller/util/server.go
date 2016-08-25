package util

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
)

// StartServer starts a server listening at addr.  Addr can be a path (for unix socket) or of the form ip:port
// Returns a channel to signal stop when closed, a channel to block on stopping, and an error if occurs.
func StartServer(addr string, endpoint http.Handler, shutdown ...func() error) (chan<- struct{}, <-chan error, error) {
	shutdownTasks := []func() error{}

	for _, onShutdown := range shutdown {
		shutdownTasks = append(shutdownTasks, onShutdown)
	}

	engineStop, engineStopped, err := runHTTP(&http.Server{Handler: endpoint, Addr: addr})
	if err != nil {
		return nil, nil, err
	}

	shutdownTasks = append(shutdownTasks, func() error {
		// close channels that others may block on for shutdown
		close(engineStop)
		err := <-engineStopped
		return err
	})

	// Pid file
	if pid, pidErr := savePidFile(); pidErr == nil {
		shutdownTasks = append(shutdownTasks, func() error {
			// remove pid file
			os.Remove(pid)
			return nil
		})
	}

	// Triggers to start shutdown sequence
	fromKernel := make(chan os.Signal, 1)

	// kill -9 is SIGKILL and is uncatchable.
	signal.Notify(fromKernel, syscall.SIGHUP)  // 1
	signal.Notify(fromKernel, syscall.SIGINT)  // 2
	signal.Notify(fromKernel, syscall.SIGQUIT) // 3
	signal.Notify(fromKernel, syscall.SIGABRT) // 6
	signal.Notify(fromKernel, syscall.SIGTERM) // 15

	fromUser := make(chan struct{})
	stopped := make(chan error)
	go func(tasks []func() error) {
		defer close(stopped)

		select {
		case <-fromKernel:
		case <-fromUser:
		}
		for _, task := range tasks {
			if err := task(); err != nil {
				stopped <- err
				return
			}
		}
		return
	}(shutdownTasks)

	return fromUser, stopped, nil
}

func ProtocolFromListenString(listen string) string {
	protocol := "tcp"
	// e.g. 0.0.0.0:80 or :80 or :8080
	if match, _ := regexp.MatchString("[a-zA-Z0-9\\.]*:[0-9]{2,}", listen); !match {
		protocol = "unix"
	}
	return protocol
}

// Runs the http server.  This server offers more control than the standard go's default http server.
// When the returned stop channel is closed, a clean shutdown and shutdown tasks are executed.
// The return value is a channel that can be used to block on.  An error is received if server shuts
// down in error; or a nil is received on a clean signalled shutdown.
func runHTTP(server *http.Server) (chan<- struct{}, <-chan error, error) {
	protocol := ProtocolFromListenString(server.Addr)
	listener, err := net.Listen(protocol, server.Addr)
	if err != nil {
		return nil, nil, err
	}

	if protocol == "unix" {
		if _, err = os.Lstat(server.Addr); err == nil {
			// Update socket filename permission
			if err := os.Chmod(server.Addr, 0777); err != nil {
				return nil, nil, err
			}
		} else {
			return nil, nil, err
		}
	}

	stop := make(chan struct{})
	stopped := make(chan error)

	userInitiated := new(bool)
	go func() {
		<-stop
		*userInitiated = true
		listener.Close()
	}()

	go func() {
		// Serve will block until an error (e.g. from shutdown, closed connection) occurs.
		err := server.Serve(listener)

		defer close(stopped)

		switch {
		case !*userInitiated && err != nil:
			panic(err)
		case *userInitiated:
			stopped <- nil
		default:
			stopped <- err
		}
	}()
	return stop, stopped, nil
}

func savePidFile() (string, error) {
	cmd := filepath.Base(os.Args[0])
	pidFile, err := os.Create(fmt.Sprintf("%s.pid", cmd))
	if err != nil {
		return "", err
	}
	defer pidFile.Close()
	fmt.Fprintf(pidFile, "%d", os.Getpid())
	return pidFile.Name(), nil
}
