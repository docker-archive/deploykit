package compute

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

// Test that the client can create an instance.
func TestSecurityIPListsClient_CreateKey(t *testing.T) {
	server := newAuthenticatingServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Wrong HTTP method %s, expected POST", r.Method)
		}

		expectedPath := "/seciplist/"
		if r.URL.Path != expectedPath {
			t.Errorf("Wrong HTTP URL %v, expected %v", r.URL, expectedPath)
		}

		listInfo := &SecurityIPListSpec{}
		unmarshalRequestBody(t, r, listInfo)

		if listInfo.Name != "/Compute-test/test/test-list1" {
			t.Errorf("Expected name 'Compute-test/test/test-list1', was %s", listInfo.Name)
		}

		if !reflect.DeepEqual(listInfo.SecIPEntries, []string{"127.0.0.1", "168.10.0.0"}) {
			t.Errorf("Expected entries [127.0.0.1,168.10.0.0], was %s", listInfo.SecIPEntries)
		}

		w.Write([]byte(exampleCreateSecurityIPListResponse))
		w.WriteHeader(201)
	})

	defer server.Close()
	client := getStubSecurityIPListsClient(server)

	info, err := client.CreateSecurityIPList("test-list1", []string{"127.0.0.1", "168.10.0.0"})
	if err != nil {
		t.Fatalf("Create security ip list request failed: %s", err)
	}

	if info.Name != "es_iplist" {
		t.Errorf("Expected name 'es_iplist', was %s", info.Name)
	}
}

func getStubSecurityIPListsClient(server *httptest.Server) *SecurityIPListsClient {
	endpoint, _ := url.Parse(server.URL)
	client := NewComputeClient("test", "test", "test", endpoint)
	authenticatedClient, _ := client.Authenticate()
	return authenticatedClient.SecurityIPLists()
}

var exampleCreateSecurityIPListResponse = `
{
  "secipentries": [
    "46.16.56.0/21",
    "46.6.0.0/16"
  ],
  "name": "/Compute-acme/jack.jones@example.com/es_iplist",
  "uri": "https://api.compute.us0.oraclecloud.com/seciplist/Compute-acme/jack.jones@example.com/es_iplist"
}
`
