package compute

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

// Test that the client can create a storage volume.
func TestStorageVolumeClient_CreateStorageVolume(t *testing.T) {
	server := newAuthenticatingServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("Wrong HTTP method %s, expected POST", r.Method)
		}

		expectedPath := "/storage/volume/"
		if r.URL.Path != expectedPath {
			t.Fatalf("Wrong HTTP URL %v, expected %v", r.URL, expectedPath)
		}

		spec := &StorageVolumeSpec{}
		unmarshalRequestBody(t, r, spec)

		if spec.Size != "15G" {
			t.Fatalf("Expected spec size of 15G, was %s", spec.Size)
		}
		w.WriteHeader(201)
	})

	defer server.Close()
	sv := getStubStorageVolumeClient(server)

	err := sv.CreateStorageVolume(sv.NewStorageVolumeSpec("15G", []string{}, "myVolume"))
	if err != nil {
		t.Fatalf("Create storage volume request failed: %s", err)
	}
}

func TestStorageVolumeClient_GetStorageVolume(t *testing.T) {
	server := newAuthenticatingServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatalf("Expected GET request, was %s", r.Method)
		}

		if r.URL.String() != "/storage/volume/Compute-test/test/myVolume/" {
			t.Fatalf("Expected request to /storage/volume/Compute-test/test/myVolume/, path was %s", r.URL.String())
		}

		svr := "{\"result\":[{\"name\":\"/Compute-test/test/test\",\"size\":\"16G\",\"status\":\"Online\"}]}"

		w.Write([]byte(svr))
		w.WriteHeader(200)
	})

	defer server.Close()
	sv := getStubStorageVolumeClient(server)

	info, err := sv.GetStorageVolume("myVolume")
	if err != nil {
		t.Fatal(err)
	}

	if len(info.Result) == 0 {
		t.Fatal("Expected StorageVolumeInfo in result, but was empty.")
	}

	if info.Result[0].Size != "16G" {
		t.Fatalf("Expected info with size of 16G, was %s", info.Result[0].Size)
	}
}

func TestStorageVolumeClient_WaitForStorageVolumeOnline(t *testing.T) {
	server := serverThatReturnsOnlineStorageVolumeAfterThreeSeconds(t)

	defer server.Close()
	sv := getStubStorageVolumeClient(server)

	info, err := sv.WaitForStorageVolumeOnline("test", 10)
	if err != nil {
		t.Fatalf("Wait for storage volume online request failed: %s", err)
	}

	if info.Status != "Online" {
		fmt.Println(info)
		t.Fatalf("Status of retrieved storage volume info was %s, expected 'Online'", info.Status)
	}
}

func TestStorageVolumeClient_WaitForStorageVolumeOnlineTimeout(t *testing.T) {
	server := serverThatReturnsOnlineStorageVolumeAfterThreeSeconds(t)

	defer server.Close()
	sv := getStubStorageVolumeClient(server)

	_, err := sv.WaitForStorageVolumeOnline("test", 3)
	if err == nil {
		t.Fatal("Expected timeout error")
	}
}

func TestStorageVolumeClient_UpdateStorageVolume(t *testing.T) {
	server := newAuthenticatingServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			svr := "{\"result\":[{\"name\":\"/Compute-test/test/test\",\"size\":\"16G\",\"status\":\"Online\"}]}"

			w.Write([]byte(svr))
			w.WriteHeader(200)
			return
		}

		if r.URL.String() != "/storage/volume/Compute-test/test/myVolume/" {
			t.Errorf("Expected request to foo, path was %s", r.URL.String())
		}

		info := &StorageVolumeInfo{}
		unmarshalRequestBody(t, r, info)

		if info.Size != "12G" {
			t.Errorf("Expected updated storage to be 12G, was %s", info.Size)
		}

		if info.Description != "updated description" {
			t.Errorf("Expected description to be 'updated description', was %s", info.Description)
		}

		if !reflect.DeepEqual(info.Tags, []string{"foo", "bar"}) {
			t.Errorf("Expected updated tags to be [foo, bar], was %s", info.Tags)
		}
		w.WriteHeader(200)
	})

	defer server.Close()
	sv := getStubStorageVolumeClient(server)

	err := sv.UpdateStorageVolume("myVolume", "12G", "updated description", []string{"foo", "bar"})
	if err != nil {
		t.Fatal(err)
	}
}

func getStubStorageVolumeClient(server *httptest.Server) *StorageVolumeClient {
	endpoint, _ := url.Parse(server.URL)
	client := NewComputeClient("test", "test", "test", endpoint)
	authenticatedClient, _ := client.Authenticate()
	return authenticatedClient.StorageVolumes()
}

func serverThatReturnsOnlineStorageVolumeAfterThreeSeconds(t *testing.T) *httptest.Server {
	count := 0
	return newAuthenticatingServer(func(w http.ResponseWriter, r *http.Request) {
		var status string
		if count < 3 {
			status = "Initializing"
		} else {
			status = "Online"
		}
		count++
		svr := fmt.Sprintf(
			"{\"result\":[{\"name\":\"/Compute-test/test/test\",\"size\":\"16G\",\"status\":\"%s\"}]}", status)

		w.Write([]byte(svr))
		w.WriteHeader(200)
	})
}
