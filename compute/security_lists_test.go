package compute

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// Test that the client can create an instance.
func TestSecurityListsClient_CreateKey(t *testing.T) {
	server := newAuthenticatingServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Wrong HTTP method %s, expected POST", r.Method)
		}

		expectedPath := "/seclist/"
		if r.URL.Path != expectedPath {
			t.Errorf("Wrong HTTP URL %v, expected %v", r.URL, expectedPath)
		}

		listInfo := &SecurityListSpec{}
		unmarshalRequestBody(t, r, listInfo)

		if listInfo.Name != "/Compute-test/test/test-list1" {
			t.Errorf("Expected name 'test-list1', was %s", listInfo.Name)
		}

		if listInfo.Policy != "DENY" {
			t.Errorf("Expected policy 'DENY', was %s", listInfo.Policy)
		}

		if listInfo.OutboundCIDRPolicy != "PERMIT" {
			t.Errorf("Expected outbound CIDR policy 'PERMIT', was %s", listInfo.OutboundCIDRPolicy)
		}

		w.Write([]byte(exampleCreateSecurityListResponse))
		w.WriteHeader(201)
	})

	defer server.Close()
	client := getStubSecurityListsClient(server)

	info, err := client.CreateSecurityList("test-list1", "DENY", "PERMIT")
	if err != nil {
		t.Fatalf("Create security list request failed: %s", err)
	}

	if info.Name != "allowed_video_servers" {
		t.Errorf("Expected name 'allowed_video_servers', was %s", info.Name)
	}
}

func getStubSecurityListsClient(server *httptest.Server) *SecurityListsClient {
	endpoint, _ := url.Parse(server.URL)
	client := NewComputeClient("test", "test", "test", endpoint)
	authenticatedClient, _ := client.Authenticate()
	return authenticatedClient.SecurityLists()
}

var exampleCreateSecurityListResponse = `
{
  "account": "/Compute-acme/default",
  "name": "/Compute-acme/jack.jones@example.com/allowed_video_servers",
  "uri": "https://api.compute.us0.oraclecloud.com/seclist/Compute-acme/jack.jones@example.com/es_list",
  "outbound_cidr_policy": "DENY",
  "policy": "PERMIT"
}
`
