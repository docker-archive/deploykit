package compute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

func newAuthenticatingServer(handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Received request: %s, %s\n", r.Method, r.URL)

		if r.URL.Path == "/authenticate/" {
			http.SetCookie(w, &http.Cookie{Name: "testAuthCookie", Value: "cookie value"})
			w.WriteHeader(200)
		} else {
			handler(w, r)
		}
	}))
}

func getAuthenticatedClient() (*AuthenticatedClient, error) {
	if os.Getenv("OPC_ENDPOINT") == "" {
		panic("OPC_ENDPOINT not set in environment")
	}

	endpoint, err := url.Parse(os.Getenv("OPC_ENDPOINT"))
	if err != nil {
		return nil, err
	}

	client := NewComputeClient(
		os.Getenv("OPC_IDENTITY_DOMAIN"),
		os.Getenv("OPC_USERNAME"),
		os.Getenv("OPC_PASSWORD"),
		endpoint,
	)

	return client.Authenticate()
}

func unmarshalRequestBody(t *testing.T, r *http.Request, target interface{}) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	err := json.Unmarshal(buf.Bytes(), target)
	if err != nil {
		t.Fatalf("Error marshalling request: %s", err)
	}
	io.Copy(os.Stdout, buf)
	fmt.Println()
}

func marshalToBytes(target interface{}) []byte {
	marshalled, err := json.Marshal(target)
	if err != nil {
		panic(err)
	}
	buf := new(bytes.Buffer)
	buf.Read(marshalled)
	io.Copy(os.Stdout, buf)
	fmt.Println()
	return marshalled
}
