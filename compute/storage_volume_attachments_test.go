package compute

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// Test that the client can create an instance.
func TestStorageAttachmentsClient_GetStorageAttachmentsForInstance(t *testing.T) {
	server := newAuthenticatingServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Wrong HTTP method %s, expected GET", r.Method)
		}

		expectedPath := "/storage/attachment/Compute-test/test/?state=attached&instance_name=/Compute-test/test/test-instance/test-id"
		if r.URL.String() != expectedPath {
			t.Errorf("Wrong HTTP URL %v, expected %v", r.URL, expectedPath)
		}

		w.Write([]byte(exampleGetStorageAttachmentsResponse))
		w.WriteHeader(200)
	})

	defer server.Close()
	client := getStubStorageAttachmentsClient(server)

	_, err := client.GetStorageAttachmentsForInstance(&InstanceName{
		Name: "test-instance",
		ID:   "test-id",
	})

	if err != nil {
		t.Fatalf("Get security attachments request failed: %s", err)
	}
}

func getStubStorageAttachmentsClient(server *httptest.Server) *StorageAttachmentsClient {
	endpoint, _ := url.Parse(server.URL)
	client := NewComputeClient("test", "test", "test", endpoint)
	authenticatedClient, _ := client.Authenticate()
	return authenticatedClient.StorageAttachments()
}

var exampleGetStorageAttachmentsResponse = `
{
"result": [
  {
    "index": 5,
    "account": null,
    "storage_volume_name": "/Compute-acme/jack.jones@example.com/data",
    "hypervisor": null,
    "uri": "https://api.compute.us0.oraclecloud.com/storage/attachment/Compute-acme/jack.jones@example.com/01fa297e-e7e1-4501-84d3-402ccc33e66d/10bf639f-bb78-462a-b5ac-eeb0474771a0",
    "instance_name": "/Compute-acme/jack.jones@example.com/01fa297e-e7e1-4501-84d3-402ccc33e66d",
    "state": "attached",
    "readonly": false,
    "name": "/Compute-acme/jack.jones@example.com/01fa297e-e7e1-4501-84d3-402ccc33e66d/10bf639f-bb78-462a..."
  },
  {
    "index": 1,
    "account": null,
    "storage_volume_name": "/Compute-acme/jack.jones@example.com/boot",
    "hypervisor": null,
    "uri": "https://api.compute.us0.oraclecloud.com/storage/attachment/Compute-acme/jack.jones@example.com/01fa297e-e7e1-4501-84d3-402ccc33e66d/4aa33097-b085-4484-a909-a6a0a5955c05",
    "instance_name": "/Compute-acme/jack.jones@example.com/01fa297e-e7e1-4501-84d3-402ccc33e66d",
    "state": "attached",
    "readonly": false,
    "name": "/Compute-acme/jack.jones@example.com/01fa297e-e7e1-4501-84d3-402ccc33e66d/4aa33097-b085-4484..."
  }
 ]
}
`
