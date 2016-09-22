package util

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func respond(t *testing.T, resp http.ResponseWriter, obj interface{}) {
	buff, err := json.Marshal(obj)
	require.NoError(t, err)
	resp.Write(buff)
	return
}

func read(t *testing.T, req *http.Request, obj interface{}) {
	defer req.Body.Close()
	err := json.NewDecoder(req.Body).Decode(obj)
	require.NoError(t, err)
	return
}

type testRequest struct {
	Name  string
	Count int
}

type testResponse struct {
	Name   string
	Status bool
}

func TestTCPClient(t *testing.T) {

	serverReq := testRequest{
		Name:  "client",
		Count: 100,
	}

	serverResp := testResponse{
		Name:   "server",
		Status: true,
	}

	router := mux.NewRouter()
	router.HandleFunc("/test", func(resp http.ResponseWriter, req *http.Request) {

		input := testRequest{}
		read(t, req, &input)
		require.Equal(t, serverReq, input)

		respond(t, resp, serverResp)
		return
	}).Methods("POST")

	dir := os.TempDir()
	stop, errors, err := StartServer("tcp://:4321"+dir, router)

	require.NoError(t, err)
	require.NotNil(t, stop)
	require.NotNil(t, errors)

	client, err := NewClient("tcp://localhost:4321")
	require.NoError(t, err)

	response := testResponse{}
	_, err = client.Call("post", "/test", serverReq, &response)

	require.NoError(t, err)
	require.Equal(t, serverResp, response)

	// Now we stop the server
	close(stop)
}

func TestUnixClient(t *testing.T) {

	serverReq := testRequest{
		Name:  "unix-client",
		Count: 1000,
	}

	serverResp := testResponse{
		Name:   "unix-server",
		Status: true,
	}

	router := mux.NewRouter()
	router.HandleFunc("/test", func(resp http.ResponseWriter, req *http.Request) {

		input := testRequest{}
		read(t, req, &input)
		require.Equal(t, serverReq, input)

		respond(t, resp, serverResp)
		return
	}).Methods("POST")

	dir := os.TempDir()
	listen := "unix://" + filepath.Join(dir, "server.sock")
	stop, errors, err := StartServer(listen, router)

	require.NoError(t, err)
	require.NotNil(t, stop)
	require.NotNil(t, errors)

	client, err := NewClient(listen)
	require.NoError(t, err)

	response := testResponse{}
	_, err = client.Call("post", "/test", serverReq, &response)

	require.NoError(t, err)
	require.Equal(t, serverResp, response)

	// Now we stop the server
	close(stop)
}
