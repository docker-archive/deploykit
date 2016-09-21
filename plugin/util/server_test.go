package util

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type handler int

func TestStartServer(t *testing.T) {

	servedCall := make(chan struct{})

	router := mux.NewRouter()
	router.HandleFunc("/test", func(resp http.ResponseWriter, req *http.Request) {
		close(servedCall)
		return
	})
	ranShutdown := make(chan struct{})

	dir := os.TempDir()
	stop, errors, err := StartServer("tcp://:4321"+dir, router, func() error {
		close(ranShutdown)
		return nil
	})

	require.NoError(t, err)
	require.NotNil(t, stop)
	require.NotNil(t, errors)

	client := &http.Client{}
	resp, err := client.Get("http://localhost:4321/test")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	<-servedCall

	pidfile := filepath.Join(dir, "tcp::4321")
	// check that the pid files exist:
	pid, err := ioutil.ReadFile(pidfile)
	require.NoError(t, err)

	t.Log("pid=", string(pid))
	require.NotEqual(t, 0, pid)

	// Now we stop the server
	close(stop)
	<-ranShutdown

	// We shouldn't block here.
	<-errors

	// ensure cleaning up of pidfile
	_, err = os.Stat(pidfile)
	require.True(t, os.IsNotExist(err))
}

func TestStartServerUnixSocket(t *testing.T) {

	servedCall := make(chan struct{})

	router := mux.NewRouter()
	router.HandleFunc("/test", func(resp http.ResponseWriter, req *http.Request) {
		close(servedCall)
		return
	})
	ranShutdown := make(chan struct{})

	socket := filepath.Join(os.TempDir(), fmt.Sprintf("%d.sock", time.Now().Unix()))
	stop, errors, err := StartServer("unix://"+socket, router, func() error {
		close(ranShutdown)
		return nil
	})

	require.NoError(t, err)

	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(proto, addr string) (conn net.Conn, err error) {
				return net.Dial("unix", socket)
			},
		},
	}
	resp, err := client.Get("http://local/test")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	<-servedCall

	// check that the pid files exist:
	pidfile := strings.Split(socket, ".sock")[0] + ".pid"
	pid, err := ioutil.ReadFile(pidfile)
	require.NoError(t, err)

	t.Log("pid=", string(pid))
	require.NotEqual(t, 0, pid)

	// Now we stop the server
	close(stop)
	<-ranShutdown

	// We shouldn't block here.
	<-errors

	// ensure cleaning up of pidfile
	_, err = os.Stat(pidfile)
	require.True(t, os.IsNotExist(err))
}
