package compute

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

// Test that the client can create an instance.
func TestInstanceClient_CreateInstance(t *testing.T) {
	server := newAuthenticatingServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Wrong HTTP method %s, expected POST", r.Method)
		}

		expectedPath := "/launchplan/"
		if r.URL.Path != expectedPath {
			t.Errorf("Wrong HTTP URL %v, expected %v", r.URL, expectedPath)
		}

		plan := &LaunchPlan{}
		unmarshalRequestBody(t, r, plan)

		spec := plan.Instances[0]

		if spec.Name != "/Compute-test/test/name" {
			t.Errorf("Expected name 'name', was %s", spec.Name)
		}

		if spec.Label != "label" {
			t.Errorf("Expected label 'label', was %s", spec.Label)
		}

		if spec.Shape != "shape" {
			t.Errorf("Expected shape 'shape', was %s", spec.Shape)
		}

		if spec.ImageList != "imagelist" {
			t.Errorf("Expected imagelist 'imagelist', was %s", spec.ImageList)
		}

		if !reflect.DeepEqual(spec.SSHKeys, []string{"/Compute-test/test/foo", "/Compute-test/test/bar"}) {
			t.Errorf("Expected sshkeys ['/Compute-test/test/foo', '/Compute-test/test/bar'], was %s", spec.SSHKeys)
		}

		w.Write([]byte(exampleCreateResponse))
		w.WriteHeader(201)
	})

	defer server.Close()
	iv := getStubInstancesClient(server)

	id, err := iv.LaunchInstance("name", "label", "shape", "imagelist", nil, nil, []string{"foo", "bar"}, map[string]interface{}{
		"attr1": 12,
		"attr2": map[string]interface{}{
			"inner_attr1": "foo",
		},
	})

	if err != nil {
		t.Fatalf("Create storage volume request failed: %s", err)
	}

	expected := "437b72fd-b870-47b1-9c01-7a2812bbe30c"
	if id.ID != expected {
		t.Errorf("Expected id %s, was %s", expected, id.ID)
	}
}

// Test that the client can create an instance.
func TestInstanceClient_RetrieveInstance(t *testing.T) {
	server := newAuthenticatingServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Wrong HTTP method %s, expected GET", r.Method)
		}

		expectedPath := "/instance/Compute-test/test/test-instance/test-id"
		if r.URL.Path != expectedPath {
			t.Errorf("Wrong HTTP URL %v, expected %v", r.URL, expectedPath)
		}

		w.Write([]byte(exampleRetrieveResponse))
		w.WriteHeader(201)
	})

	defer server.Close()
	iv := getStubInstancesClient(server)

	info, err := iv.GetInstance(&InstanceName{Name: "test-instance", ID: "test-id"})
	if err != nil {
		t.Fatalf("%s", err)
	}
	if info.State != "running" {
		t.Errorf("Expected state 'running', was %s", info.State)
	}
	if info.SSHKeys[0] != "acme-prod-admin" {
		t.Errorf("Expected ssh key 'acme-prod-admin', was %s", info.SSHKeys[0])
	}
}

func getStubInstancesClient(server *httptest.Server) *InstancesClient {
	endpoint, _ := url.Parse(server.URL)
	client := NewComputeClient("test", "test", "test", endpoint)
	authenticatedClient, _ := client.Authenticate()
	return authenticatedClient.Instances()
}

var exampleCreateResponse = `
{
  "relationships": [

  ],
  "instances": [
    {
      "domain": "compute-acme...",
      "placement_requirements": [
        "/system/compute/placement/default",
        "/system/compute/allow_instances"
      ],
      "ip": "0.0.0.0",
      "fingerprint": "",
      "site": "",
      "last_state_change_time": null,
      "cluster": null,
      "shape": "oc3",
      "vethernets": null,
      "imagelist": "/oracle/public/oel_6.4_2GB_v1",
      "image_format": "raw",
      "id": "437b72fd-b870-47b1-9c01-7a2812bbe30c",
      "cluster_uri": null,
      "networking": {
        "eth0": {
          "model": "",
          "seclists": [
            "/Compute-acme/default/default"
          ],
          "dns": [
            "d9dd5d.compute-acme.oraclecloud.internal."
          ],
          "nat": null,
          "vethernet": "/oracle/public/default"
        }
      },
      "seclist_associations": null,
      "hostname": "d9dd5d.compute-acme.oraclecloud.internal.",
      "state": "queued",
      "disk_attach": "",
      "label": "dev-vm",
      "priority": "/oracle/public/default",
      "platform": "linux",
      "quota_reservation": null,
      "suspend_file": null,
      "node": null,
      "resource_requirements": {
        "compressed_size": 727212045,
        "ram": 7680,
        "cpus": 2.0,
        "root_disk_size": 0,
        "io": 200,
        "decompressed_size": 2277507072
      },
      "virtio": null,
      "vnc": "",
      "storage_attachments": [],
      "start_time": "2016-04-06T18:03:12Z",
      "storage_attachment_associations": [],
      "quota": "/Compute-acme",
      "vnc_key": null,
      "numerical_priority": 100,
      "suspend_requested": false,
      "entry": 1,
      "error_reason": "",
      "nat_associations": null,
      "sshkeys": ["/Compute-acme/jack.jones@example.com/dev-key1"],
      "tags": [],
      "resolvers": null,
      "metrics": null,
      "account": "/Compute-acme/default",
      "node_uuid": null,
      "name": "/Compute-acme/jack.jones@example.com/dev-vm/437b72fd-b870-47b1-9c01-7a2812bbe30c",
      "vcable_id": null,
      "hypervisor": {"mode": "hvm"},
      "uri": "https://api.compute.us0.oraclecloud.com/instance/Compute-acme/jack.jones@example.com/dev-vm/437b72fd-b870-47b1-9c01-7a2812bbe30c",
      "console": null,
      "reverse_dns": true,
      "delete_requested": null,
      "hypervisor_type": null,
      "attributes": {
        "sshkeys": ["ssh-rsa AAAAB3..."]
      },
      "boot_order": [],
      "last_seen": null
    }
  ]
}`

var exampleRetrieveResponse = `
{
"domain": "acme...",
"placement_requirements": [
"/system/compute/placement/default",
"/system/compute/allow_instances"
],
"ip": "10...",
"site": "",
"shape": "oc5",
"imagelist": "/oracle/public/oel_6.4_60GB",
"attributes": {
"network": {
"nimbula_vcable-eth0": {
"vethernet_id": "0",
"vethernet": "/oracle/public/default",
"address": [
"c6:b0:09:f4:bc:c0",
"0.0.0.0"
],
"model": "",
"vethernet_type": "vlan",
"id": "/Compute-acme/jack.jones@example.com/016e75e7-e911-42d1-bfe1-6a7f1b3f7908",
"dhcp_options": []
}
},
"dns": {
"domain": "acme...",
"hostname": "d06886.compute-acme...",
"nimbula_vcable-eth0": "d06886.acme..."
},
"sshkeys": [
"ssh-rsa AAAAB3NzaC1yc2EAAA..."
]
},
"networking": {
"eth0": {
"model": "",
"dns": [
"d06886.acme..."
],
"seclists": [
"/Compute-acme/default/default",
"/Compute-acme/jack.jones@example.com/prod-ng"
],
"vethernet": "/oracle/public/default",
"nat": "ipreservation:/Compute-acme/jack.jones@example.com/prod-vm1"
}
},
"hostname": "d06886.acme...",
"quota_reservation": "/Compute-acme/ffc8e6d4-8f93-41f3-a062-bdbb042c3191",
"disk_attach": "",
"label": "Production instance 1",
"priority": "/oracle/public/default",
"state": "running",
"vnc": "10...",
"storage_attachments": [
{
"index": 1,
"storage_volume_name": "/Compute-acme/jack.jones@example.com/prod-vol1",
"name": "/Compute-acme/admin/dev1/f653a677-b566-4f92-8e93-71d47b364119/f1a67244-9abc-45d5-af69-8..."
}
],
"start_time": "2014-06-24T17:51:35Z",
"quota": "/acme",
"fingerprint": "19:c4:3f:2d:dc:76:b1:06:e8:88:bd:7f:a3:3b:3c:93",
"error_reason": "",
"sshkeys": [
"/Compute-acme/jack.jones@example.com/acme-prod-admin"
],
"tags": [
"prod2"
],
"resolvers": null,
"account": "/Compute-acme/default",
"name": "/Compute-acme/jack.jones@example.com/dev1/f653a677-b566-4f92-8e93-71d47b364119",
"vcable_id": "/Compute-acme/jack.jones@example.com/016e75e7-e911-42d1-bfe1-6a7f1b3f7908",
"uri": "http://10....",
"reverse_dns": true,
"entry": 1,
"boot_order": []
}
`
