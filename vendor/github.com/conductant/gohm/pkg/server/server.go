package server

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func Start(port interface{}, endpoint http.Handler,
	onShutdown func() error, timeout time.Duration) (chan<- int, <-chan error) {

	shutdownTasks := make(chan func() error, 100)

	// Custom shutdown task
	shutdownTasks <- onShutdown

	addr := ""
	switch port := port.(type) {
	case int:
		addr = fmt.Sprintf(":%d", port)
	case string:
		if p, err := strconv.Atoi(port); err == nil {
			addr = fmt.Sprintf(":%d", p)
		} else if fp, err := filepath.Abs(port); err == nil {
			addr = fp
		} else {
			panic(err)
		}
	}

	engineStop, engineStopped := RunServer(&http.Server{Handler: endpoint, Addr: addr})
	shutdownTasks <- func() error {
		engineStop <- 1
		err := <-engineStopped
		return err
	}

	// Pid file
	if pid, pidErr := savePidFile(fmt.Sprintf("%v", port)); pidErr == nil {
		// Clean up pid file
		shutdownTasks <- func() error {
			os.Remove(pid)
			return nil
		}
	}
	shutdownTasks <- nil // stop on this

	// Triggers to start shutdown sequence
	fromKernel := make(chan os.Signal, 1)

	// kill -9 is SIGKILL and is uncatchable.
	signal.Notify(fromKernel, syscall.SIGHUP)  // 1
	signal.Notify(fromKernel, syscall.SIGINT)  // 2
	signal.Notify(fromKernel, syscall.SIGQUIT) // 3
	signal.Notify(fromKernel, syscall.SIGABRT) // 6
	signal.Notify(fromKernel, syscall.SIGTERM) // 15

	fromUser := make(chan int)
	stopped := make(chan error)
	go func() {
		select {
		case <-fromKernel:
		case <-fromUser:
		}
		for {
			task, ok := <-shutdownTasks
			if !ok || task == nil {
				break
			}
			if err := task(); err != nil {
				stopped <- err
				return
			}
		}
		stopped <- nil
		return
	}()

	return fromUser, stopped
}

// Runs the http server.  This server offers more control than the standard go's default http server
// in that when a 'true' is sent to the stop channel, the listener is closed to force a clean shutdown.
// The return value is a channel that can be used to block on.  An error is received if server shuts
// down in error; or a nil is received on a clean signalled shutdown.
func RunServer(server *http.Server) (chan<- int, <-chan error) {
	protocol := "tcp"
	// e.g. 0.0.0.0:80 or :80 or :8080
	if match, _ := regexp.MatchString("[a-zA-Z0-9\\.]*:[0-9]{2,}", server.Addr); !match {
		protocol = "unix"
	}

	listener, err := net.Listen(protocol, server.Addr)
	if err != nil {
		panic(err)
	}

	stop := make(chan int)
	stopped := make(chan error)

	if protocol == "unix" {
		if _, err = os.Lstat(server.Addr); err == nil {
			// Update socket filename permission
			os.Chmod(server.Addr, 0777)
		}
	}

	userInitiated := new(bool)
	go func() {
		<-stop
		*userInitiated = true
		listener.Close()
	}()

	go func() {

		// Serve will block until an error (e.g. from shutdown, closed connection) occurs.
		err := server.Serve(listener)

		switch {
		case !*userInitiated && err != nil:
			panic(err)
		case *userInitiated:
			stopped <- nil
		default:
			stopped <- err
		}
	}()
	return stop, stopped
}

func savePidFile(args ...string) (string, error) {
	cmd := filepath.Base(os.Args[0])
	pidFile, err := os.Create(fmt.Sprintf("%s-%s.pid", cmd, strings.Join(args, "-")))
	if err != nil {
		return "", err
	}
	defer pidFile.Close()
	fmt.Fprintf(pidFile, "%d", os.Getpid())
	return pidFile.Name(), nil
}
