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
	"testing"
	"time"
)

type handler int

func TestTCPServer(t *testing.T) {

	servedCall := make(chan struct{})

	router := mux.NewRouter()
	router.HandleFunc("/test", func(resp http.ResponseWriter, req *http.Request) {
		close(servedCall)
		return
	})
	ranShutdown := make(chan struct{})

	dir := os.TempDir()
	name := "test-tcp-server"

	listen := "tcp://:4321" + filepath.Join(dir, name)
	stop, errors, err := StartServer(listen, router, func() error {
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

	crumbFile := filepath.Join(dir, name)
	// check that the pid files exist:
	urlString, err := ioutil.ReadFile(crumbFile)
	require.NoError(t, err)

	t.Log("url=", string(urlString))
	require.Equal(t, listen, string(urlString))

	// Now we stop the server
	close(stop)
	<-ranShutdown

	// We shouldn't block here.
	<-errors

	// ensure cleaning up of pidfile
	_, err = os.Stat(crumbFile)
	require.True(t, os.IsNotExist(err))
}

func TestUnixSocketServer(t *testing.T) {

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

	// Now we stop the server
	close(stop)
	<-ranShutdown

	// We shouldn't block here.
	<-errors
}
