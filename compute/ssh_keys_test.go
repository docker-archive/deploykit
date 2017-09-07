package compute

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// Test that the client can create an instance.
func TestSSHClient_CreateKey(t *testing.T) {
	server := newAuthenticatingServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Wrong HTTP method %s, expected POST", r.Method)
		}

		expectedPath := "/sshkey/"
		if r.URL.Path != expectedPath {
			t.Errorf("Wrong HTTP URL %v, expected %v", r.URL, expectedPath)
		}

		keyInfo := &SSHKeySpec{}
		unmarshalRequestBody(t, r, keyInfo)

		if keyInfo.Name != "/Compute-test/test/test-key1" {
			t.Errorf("Expected name '/Compute-test/test/test-key1', was %s", keyInfo.Name)
		}

		if keyInfo.Enabled != true {
			t.Errorf("Key %s was not enabled", keyInfo.Name)
		}

		if keyInfo.Key != "key" {
			t.Errorf("Expected key 'key', was %s", keyInfo.Key)
		}

		w.Write([]byte(exampleCreateKeyResponse))
		w.WriteHeader(201)
	})

	defer server.Close()
	client := getStubSSHKeysClient(server)

	info, err := client.CreateSSHKey("test-key1", "key", true)
	if err != nil {
		t.Fatalf("Create ssh key request failed: %s", err)
	}

	if info.Name != "test-key1" {
		t.Errorf("Expected key 'test-key1, was %s", info.Name)
	}

	expected := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDzU21CEj6JsqIMQAYwNbmZ5P2BVxA..."
	if info.Key != expected {
		t.Errorf("Expected key %s, was %s", expected, info.Key)
	}
}

func getStubSSHKeysClient(server *httptest.Server) *SSHKeysClient {
	endpoint, _ := url.Parse(server.URL)
	client := NewComputeClient("test", "test", "test", endpoint)
	authenticatedClient, _ := client.Authenticate()
	return authenticatedClient.SSHKeys()
}

var exampleCreateKeyResponse = `
{
 "enabled": false,
 "uri": "https://api.compute.us0.oraclecloud.com/sshkey/Compute-test/test/test-key1",
 "key": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDzU21CEj6JsqIMQAYwNbmZ5P2BVxA...",
 "name": "/Compute-test/test/test-key1"
}
`
