package util

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"net"
	"net/http"
	"os"
	"path/filepath"
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

	stop, errors, err := StartServer(":4321", router, func() error {
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

	// Check the pid file exists
	_, err = os.Open(fmt.Sprintf("%s.pid", filepath.Base(os.Args[0])))
	require.NoError(t, err)

	// Now we stop the server
	close(stop)
	<-ranShutdown

	// We shouldn't block here.
	<-errors
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
	stop, _, err := StartServer(socket, router, func() error {
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
}
