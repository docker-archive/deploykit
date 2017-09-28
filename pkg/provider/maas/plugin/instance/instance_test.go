package instance

import (
	"encoding/json"
	"fmt"
	"github.com/docker/infrakit/pkg/spi/instance"
	maas "github.com/juju/gomaasapi"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"
)

func setUpTest() *maas.SimpleTestServer {
	server := maas.NewSimpleServer()
	server.AddGetResponse("/api/2.0/users/?op=whoami", http.StatusOK, `"captain awesome"`)
	server.AddGetResponse("/api/2.0/version/", http.StatusOK, versionResponse)
	server.Start()
	return server
}

func updateJSONMap(t *testing.T, source string, changes map[string]interface{}) string {
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(source), &parsed)
	require.NoError(t, err)
	for key, value := range changes {
		parsed[key] = value
	}
	bytes, err := json.Marshal(parsed)
	require.NoError(t, err)
	return string(bytes)
}

func addAllocateResponse(t *testing.T, s *maas.SimpleTestServer, status int, machineRes string) string {
	constraints := make(map[string]interface{})
	allocateJSON := updateJSONMap(t, machineRes, map[string]interface{}{
		"constraints_by_type": constraints,
	})
	s.AddPostResponse("/api/2.0/machines/?op=allocate", status, allocateJSON)
	return allocateJSON
}

func addDeployResponse(t *testing.T, s *maas.SimpleTestServer, systemID string, status int, machineRes string) string {
	response := updateJSONMap(t, machineRes, map[string]interface{}{
		"status_name":    "Deploying",
		"status_message": "for testing",
	})
	s.AddPostResponse("/MAAS/api/2.0/machines/"+systemID+"/?op=deploy", status, response)
	return response
}

func newTestMaasPlugin(t *testing.T, dir string, s *maas.SimpleTestServer) instance.Plugin {
	ctl, err := maas.NewController(maas.ControllerArgs{
		BaseURL: s.URL,
		APIKey:  "fake:as:key",
	})
	require.NoError(t, err)
	authClient, err := maas.NewAuthenticatedClient(s.URL, "fake:as:key")
	require.NoError(t, err)
	maasobj := maas.NewMAAS(*authClient)
	return &maasPlugin{MaasfilesDir: dir, MaasObj: maasobj, controller: ctl}
}

func TestProvision_and_Destroy(t *testing.T) {
	testServer := setUpTest()
	defer testServer.Close()
	testServer.AddGetResponse("/api/2.0/machines/", http.StatusOK, machinesResponse)
	testServer.AddGetResponse("/api/2.0/machines/?hostname=untasted-markita", http.StatusOK, "["+machineResponse+"]")
	dir, _ := os.Getwd()
	maasPlugin := newTestMaasPlugin(t, dir, testServer)
	require.NotNil(t, maasPlugin)
	instanceSpec := instance.Spec{
		Tags: map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
		Init: "",
	}
	amr := addAllocateResponse(t, testServer, http.StatusOK, machineResponse)
	dmr := addDeployResponse(t, testServer, "4y3ha3", http.StatusOK, amr)
	testServer.AddGetResponse("/api/2.0/machines/?id=4y3ha3", http.StatusOK, "["+dmr+"]")
	testServer.AddGetResponse("/tags/label1_value1/", http.StatusOK, `{"resource_uri":"/MAAS/api/2.0/tags/label1_value1/", "name":"label1_value1", "comment":"value1"}`)
	testServer.AddPostResponse("/MAAS/api/2.0/tags/label1_value1/?op=update_nodes", http.StatusOK, `{"resource_uri":"/MAAS/api/2.0/tags/label1_value1/", "name":"label1_value1", "comment":"value1"}`)
	testServer.AddGetResponse("/tags/label2_value2/", http.StatusOK, `{"resource_uri":"/MAAS/api/2.0/tags/label2_value2/", "name":"label2_value2", "comment":"value2"}`)
	testServer.AddPostResponse("/MAAS/api/2.0/tags/label2_value2/?op=update_nodes", http.StatusOK, `{"resource_uri":"/MAAS/api/2.0/tags/label2_value2/", "name":"label2_value2", "comment":"value2"}`)
	testServer.AddPostResponse("/tags/?op=", http.StatusOK, tagsResponce)

	_, err := maasPlugin.Provision(instanceSpec)
	require.NoError(t, err)
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		machineDir := path.Join(dir, file.Name())
		systemID, err := ioutil.ReadFile(path.Join(machineDir, "MachineID"))
		require.NoError(t, err)
		if "4y3ha3" == string(systemID) {
			err := os.RemoveAll(machineDir)
			require.NoError(t, err)
		}
	}
}

func TestDescrive(t *testing.T) {
	testServer := setUpTest()
	defer testServer.Close()
	id := instance.ID("4y3ha3")
	dir, _ := os.Getwd()
	maasPlugin := newTestMaasPlugin(t, dir, testServer)
	require.NotNil(t, maasPlugin)
	tm := updateJSONMap(t, machineResponse, map[string]interface{}{
		"tag_names": []string{"label1_value1", "label2_value2"},
	})
	testServer.AddGetResponse("/api/2.0/machines/", http.StatusOK, "["+tm+"]")
	testServer.AddGetResponse("/api/2.0/machines/?id=4y3ha3", http.StatusOK, "["+tm+"]")
	testServer.AddGetResponse("/api/2.0/machines/?hostname=untasted-markita", http.StatusOK, "["+tm+"]")
	list, err := maasPlugin.DescribeInstances(map[string]string{"label1": "value1", "label2": "value2"}, false)
	require.NoError(t, err)
	require.Equal(t, id, list[0].ID)
	require.Equal(t, map[string]string{
		"label1": "value1",
		"label2": "value2",
	},
		list[0].Tags)
	require.Equal(t, instance.LogicalID("192.168.100.4"), *list[0].LogicalID)
}

var versionResponse = `{"version": "unknown", "subversion": "", "capabilities": ["networks-management", "static-ipaddresses", "ipv6-deployment-ubuntu", "devices-management", "storage-deployment-ubuntu", "network-deployment-ubuntu"]}`

var tagsResponce = `
{"name":"label1","comment":"value1"}
`

func machineWithOwnerData(data string) string {
	return fmt.Sprintf(machineOwnerDataTemplate, data)
}

const (
	machineOwnerDataTemplate = `
	{
        "netboot": false,
        "system_id": "4y3ha3",
        "ip_addresses": [
            "192.168.100.4"
        ],
        "virtualblockdevice_set": [],
        "memory": 1024,
        "cpu_count": 1,
        "hwe_kernel": "hwe-t",
        "status_action": "",
        "osystem": "ubuntu",
        "node_type_name": "Machine",
        "macaddress_set": [
            {
                "mac_address": "52:54:00:55:b6:80"
            }
        ],
        "special_filesystems": [],
        "status": 6,
        "physicalblockdevice_set": [
            {
                "path": "/dev/disk/by-dname/sda",
                "name": "sda",
                "used_for": "MBR partitioned with 1 partition",
                "partitions": [
                    {
                        "bootable": false,
                        "id": 1,
                        "path": "/dev/disk/by-dname/sda-part1",
                        "filesystem": {
                            "fstype": "ext4",
                            "mount_point": "/",
                            "label": "root",
                            "mount_options": null,
                            "uuid": "fcd7745e-f1b5-4f5d-9575-9b0bb796b752"
                        },
                        "type": "partition",
                        "resource_uri": "/MAAS/api/2.0/nodes/4y3ha3/blockdevices/34/partition/1",
                        "uuid": "6199b7c9-b66f-40f6-a238-a938a58a0adf",
                        "used_for": "ext4 formatted filesystem mounted at /",
                        "size": 8581545984
                    }
                ],
                "filesystem": null,
                "id_path": "/dev/disk/by-id/ata-QEMU_HARDDISK_QM00001",
                "resource_uri": "/MAAS/api/2.0/nodes/4y3ha3/blockdevices/34/",
                "id": 34,
                "serial": "QM00001",
                "type": "physical",
                "block_size": 4096,
                "used_size": 8586788864,
                "available_size": 0,
                "partition_table_type": "MBR",
                "uuid": null,
                "size": 8589934592,
                "model": "QEMU HARDDISK",
                "tags": [
                    "rotary"
                ]
            },
            {
                "path": "/dev/disk/by-dname/sdb",
                "name": "sdb",
                "used_for": "MBR partitioned with 1 partition",
                "partitions": [
                    {
                        "bootable": false,
                        "id": 101,
                        "path": "/dev/disk/by-dname/sdb-part1",
                        "filesystem": {
                            "fstype": "ext4",
                            "mount_point": "/home",
                            "label": "home",
                            "mount_options": null,
                            "uuid": "fcd7745e-f1b5-4f5d-9575-9b0bb796b753"
                        },
                        "type": "partition",
                        "resource_uri": "/MAAS/api/2.0/nodes/4y3ha3/blockdevices/98/partition/101",
                        "uuid": "6199b7c9-b66f-40f6-a238-a938a58a0ae0",
                        "used_for": "ext4 formatted filesystem mounted at /home",
                        "size": 8581545984
                    }
                ],
                "filesystem": null,
                "id_path": "/dev/disk/by-id/ata-QEMU_HARDDISK_QM00002",
                "resource_uri": "/MAAS/api/2.0/nodes/4y3ha3/blockdevices/98/",
                "id": 98,
                "serial": "QM00002",
                "type": "physical",
                "block_size": 4096,
                "used_size": 8586788864,
                "available_size": 0,
                "partition_table_type": "MBR",
                "uuid": null,
                "size": 8589934592,
                "model": "QEMU HARDDISK",
                "tags": [
                    "rotary"
                ]
            }
        ],
        "interface_set": [
            {
                "effective_mtu": 1500,
                "mac_address": "52:54:00:55:b6:80",
                "children": [],
                "discovered": [],
                "params": "",
                "vlan": {
                    "resource_uri": "/MAAS/api/2.0/vlans/1/",
                    "id": 1,
                    "secondary_rack": null,
                    "mtu": 1500,
                    "primary_rack": "4y3h7n",
                    "name": "untagged",
                    "fabric": "fabric-0",
                    "dhcp_on": true,
                    "vid": 0
                },
                "name": "eth0",
                "enabled": true,
                "parents": [],
                "id": 35,
                "type": "physical",
                "resource_uri": "/MAAS/api/2.0/nodes/4y3ha3/interfaces/35/",
                "tags": [],
                "links": [
                    {
                        "id": 82,
                        "ip_address": "192.168.100.4",
                        "subnet": {
                            "resource_uri": "/MAAS/api/2.0/subnets/1/",
                            "id": 1,
                            "rdns_mode": 2,
                            "vlan": {
                                "resource_uri": "/MAAS/api/2.0/vlans/1/",
                                "id": 1,
                                "secondary_rack": null,
                                "mtu": 1500,
                                "primary_rack": "4y3h7n",
                                "name": "untagged",
                                "fabric": "fabric-0",
                                "dhcp_on": true,
                                "vid": 0
                            },
                            "dns_servers": [],
                            "space": "space-0",
                            "name": "192.168.100.0/24",
                            "gateway_ip": "192.168.100.1",
                            "cidr": "192.168.100.0/24"
                        },
                        "mode": "auto"
                    }
                ]
            },
            {
                "effective_mtu": 1500,
                "mac_address": "52:54:00:55:b6:81",
                "children": [],
                "discovered": [],
                "params": "",
                "vlan": {
                    "resource_uri": "/MAAS/api/2.0/vlans/1/",
                    "id": 1,
                    "secondary_rack": null,
                    "mtu": 1500,
                    "primary_rack": "4y3h7n",
                    "name": "untagged",
                    "fabric": "fabric-0",
                    "dhcp_on": true,
                    "vid": 0
                },
                "name": "eth0",
                "enabled": true,
                "parents": [],
                "id": 99,
                "type": "physical",
                "resource_uri": "/MAAS/api/2.0/nodes/4y3ha3/interfaces/99/",
                "tags": [],
                "links": [
                    {
                        "id": 83,
                        "ip_address": "192.168.100.5",
                        "subnet": {
                            "resource_uri": "/MAAS/api/2.0/subnets/1/",
                            "id": 1,
                            "rdns_mode": 2,
                            "vlan": {
                                "resource_uri": "/MAAS/api/2.0/vlans/1/",
                                "id": 1,
                                "secondary_rack": null,
                                "mtu": 1500,
                                "primary_rack": "4y3h7n",
                                "name": "untagged",
                                "fabric": "fabric-0",
                                "dhcp_on": true,
                                "vid": 0
                            },
                            "dns_servers": [],
                            "space": "space-0",
                            "name": "192.168.100.0/24",
                            "gateway_ip": "192.168.100.1",
                            "cidr": "192.168.100.0/24"
                        },
                        "mode": "auto"
                    }
                ]
            }
        ],
        "resource_uri": "/MAAS/api/2.0/machines/4y3ha3/",
        "hostname": "untasted-markita",
        "status_name": "Deployed",
        "min_hwe_kernel": "",
        "address_ttl": null,
        "boot_interface": {
            "effective_mtu": 1500,
            "mac_address": "52:54:00:55:b6:80",
            "children": [],
            "discovered": [],
            "params": "",
            "vlan": {
                "resource_uri": "/MAAS/api/2.0/vlans/1/",
                "id": 1,
                "secondary_rack": null,
                "mtu": 1500,
                "primary_rack": "4y3h7n",
                "name": "untagged",
                "fabric": "fabric-0",
                "dhcp_on": true,
                "vid": 0
            },
            "name": "eth0",
            "enabled": true,
            "parents": [],
            "id": 35,
            "type": "physical",
            "resource_uri": "/MAAS/api/2.0/nodes/4y3ha3/interfaces/35/",
            "tags": [],
            "links": [
                {
                    "id": 82,
                    "ip_address": "192.168.100.4",
                    "subnet": {
                        "resource_uri": "/MAAS/api/2.0/subnets/1/",
                        "id": 1,
                        "rdns_mode": 2,
                        "vlan": {
                            "resource_uri": "/MAAS/api/2.0/vlans/1/",
                            "id": 1,
                            "secondary_rack": null,
                            "mtu": 1500,
                            "primary_rack": "4y3h7n",
                            "name": "untagged",
                            "fabric": "fabric-0",
                            "dhcp_on": true,
                            "vid": 0
                        },
                        "dns_servers": [],
                        "space": "space-0",
                        "name": "192.168.100.0/24",
                        "gateway_ip": "192.168.100.1",
                        "cidr": "192.168.100.0/24"
                    },
                    "mode": "auto"
                }
            ]
        },
        "power_state": "on",
        "architecture": "amd64/generic",
        "power_type": "virsh",
        "distro_series": "trusty",
        "tag_names": [
           "virtual", "magic"
        ],
        "disable_ipv4": false,
        "status_message": "From 'Deploying' to 'Deployed'",
        "swap_size": null,
        "blockdevice_set": [
            {
                "path": "/dev/disk/by-dname/sda",
                "partition_table_type": "MBR",
                "name": "sda",
                "used_for": "MBR partitioned with 1 partition",
                "partitions": [
                    {
                        "bootable": false,
                        "id": 1,
                        "path": "/dev/disk/by-dname/sda-part1",
                        "filesystem": {
                            "fstype": "ext4",
                            "mount_point": "/",
                            "label": "root",
                            "mount_options": null,
                            "uuid": "fcd7745e-f1b5-4f5d-9575-9b0bb796b752"
                        },
                        "type": "partition",
                        "resource_uri": "/MAAS/api/2.0/nodes/4y3ha3/blockdevices/34/partition/1",
                        "uuid": "6199b7c9-b66f-40f6-a238-a938a58a0adf",
                        "used_for": "ext4 formatted filesystem mounted at /",
                        "size": 8581545984
                    }
                ],
                "filesystem": null,
                "id_path": "/dev/disk/by-id/ata-QEMU_HARDDISK_QM00001",
                "resource_uri": "/MAAS/api/2.0/nodes/4y3ha3/blockdevices/34/",
                "id": 34,
                "serial": "QM00001",
                "block_size": 4096,
                "type": "physical",
                "used_size": 8586788864,
                "tags": [
                    "rotary"
                ],
                "available_size": 0,
                "uuid": null,
                "size": 8589934592,
                "model": "QEMU HARDDISK"
            },
            {
                "path": "/dev/disk/by-dname/sdb",
                "name": "sdb",
                "used_for": "MBR partitioned with 1 partition",
                "partitions": [
                    {
                        "bootable": false,
                        "id": 101,
                        "path": "/dev/disk/by-dname/sdb-part1",
                        "filesystem": {
                            "fstype": "ext4",
                            "mount_point": "/home",
                            "label": "home",
                            "mount_options": null,
                            "uuid": "fcd7745e-f1b5-4f5d-9575-9b0bb796b753"
                        },
                        "type": "partition",
                        "resource_uri": "/MAAS/api/2.0/nodes/4y3ha3/blockdevices/98/partition/101",
                        "uuid": "6199b7c9-b66f-40f6-a238-a938a58a0ae0",
                        "used_for": "ext4 formatted filesystem mounted at /home",
                        "size": 8581545984
                    }
                ],
                "filesystem": null,
                "id_path": "/dev/disk/by-id/ata-QEMU_HARDDISK_QM00002",
                "resource_uri": "/MAAS/api/2.0/nodes/4y3ha3/blockdevices/98/",
                "id": 98,
                "serial": "QM00002",
                "type": "physical",
                "block_size": 4096,
                "used_size": 8586788864,
                "available_size": 0,
                "partition_table_type": "MBR",
                "uuid": null,
                "size": 8589934592,
                "model": "QEMU HARDDISK",
                "tags": [
                    "rotary"
                ]
            }
        ],
        "zone": {
            "description": "",
            "resource_uri": "/MAAS/api/2.0/zones/default/",
            "name": "default"
        },
        "fqdn": "untasted-markita.maas",
        "storage": 8589.934592,
        "node_type": 0,
        "boot_disk": null,
        "owner": "thumper",
        "domain": {
            "id": 0,
            "name": "maas",
            "resource_uri": "/MAAS/api/2.0/domains/0/",
            "resource_record_count": 0,
            "ttl": null,
            "authoritative": true
        },
        "owner_data": %s
    }
`

	createDeviceResponse = `
{
	"zone": {
		"description": "",
		"resource_uri": "/MAAS/api/2.0/zones/default/",
		"name": "default"
	},
	"domain": {
		"resource_record_count": 0,
		"resource_uri": "/MAAS/api/2.0/domains/0/",
		"authoritative": true,
		"name": "maas",
		"ttl": null,
		"id": 0
	},
	"node_type_name": "Device",
	"address_ttl": null,
	"hostname": "furnacelike-brittney",
	"node_type": 1,
	"resource_uri": "/MAAS/api/2.0/devices/4y3haf/",
	"ip_addresses": ["192.168.100.11"],
	"owner": "thumper",
	"tag_names": [],
	"fqdn": "furnacelike-brittney.maas",
	"system_id": "4y3haf",
	"parent": "4y3ha3",
	"interface_set": [
		{
			"resource_uri": "/MAAS/api/2.0/nodes/4y3haf/interfaces/48/",
			"type": "physical",
			"mac_address": "78:f0:f1:16:a7:46",
			"params": "",
			"discovered": null,
			"effective_mtu": 1500,
			"id": 48,
			"children": [],
			"links": [],
			"name": "eth0",
			"vlan": {
				"secondary_rack": null,
				"dhcp_on": true,
				"fabric": "fabric-0",
				"mtu": 1500,
				"primary_rack": "4y3h7n",
				"resource_uri": "/MAAS/api/2.0/vlans/1/",
				"external_dhcp": null,
				"name": "untagged",
				"id": 1,
				"vid": 0
			},
			"tags": [],
			"parents": [],
			"enabled": true
		}
	]
}
`
)

var (
	machineResponse = machineWithOwnerData(`{
            "fez": "phil fish",
            "frog-fractions": "jim crawford"
        }
`)

	machinesResponse = "[" + machineResponse + `,
    {
        "netboot": true,
        "system_id": "4y3ha4",
        "ip_addresses": [],
        "virtualblockdevice_set": [],
        "memory": 1024,
        "cpu_count": 1,
        "hwe_kernel": "",
        "status_action": "",
        "osystem": "",
        "node_type_name": "Machine",
        "macaddress_set": [
            {
                "mac_address": "52:54:00:33:6b:2c"
            }
        ],
        "special_filesystems": [],
        "status": 4,
        "physicalblockdevice_set": [
            {
                "path": "/dev/disk/by-dname/sda",
                "name": "sda",
                "used_for": "MBR partitioned with 1 partition",
                "partitions": [
                    {
                        "bootable": false,
                        "id": 2,
                        "path": "/dev/disk/by-dname/sda-part1",
                        "filesystem": {
                            "fstype": "ext4",
                            "mount_point": "/",
                            "label": "root",
                            "mount_options": null,
                            "uuid": "7a0e75a8-0bc6-456b-ac92-4769e97baf02"
                        },
                        "type": "partition",
                        "resource_uri": "/MAAS/api/2.0/nodes/4y3ha4/blockdevices/35/partition/2",
                        "uuid": "6fe782cf-ad1a-4b31-8beb-333401b4d4bb",
                        "used_for": "ext4 formatted filesystem mounted at /",
                        "size": 8581545984
                    }
                ],
                "filesystem": null,
                "id_path": "/dev/disk/by-id/ata-QEMU_HARDDISK_QM00001",
                "resource_uri": "/MAAS/api/2.0/nodes/4y3ha4/blockdevices/35/",
                "id": 35,
                "serial": "QM00001",
                "type": "physical",
                "block_size": 4096,
                "used_size": 8586788864,
                "available_size": 0,
                "partition_table_type": "MBR",
                "uuid": null,
                "size": 8589934592,
                "model": "QEMU HARDDISK",
                "tags": [
                    "rotary"
                ]
            }
        ],
        "interface_set": [
            {
                "effective_mtu": 1500,
                "mac_address": "52:54:00:33:6b:2c",
                "children": [],
                "discovered": [],
                "params": "",
                "vlan": {
                    "resource_uri": "/MAAS/api/2.0/vlans/1/",
                    "id": 1,
                    "secondary_rack": null,
                    "mtu": 1500,
                    "primary_rack": "4y3h7n",
                    "name": "untagged",
                    "fabric": "fabric-0",
                    "dhcp_on": true,
                    "vid": 0
                },
                "name": "eth0",
                "enabled": true,
                "parents": [],
                "id": 39,
                "type": "physical",
                "resource_uri": "/MAAS/api/2.0/nodes/4y3ha4/interfaces/39/",
                "tags": [],
                "links": [
                    {
                        "id": 67,
                        "mode": "auto",
                        "subnet": {
                            "resource_uri": "/MAAS/api/2.0/subnets/1/",
                            "id": 1,
                            "rdns_mode": 2,
                            "vlan": {
                                "resource_uri": "/MAAS/api/2.0/vlans/1/",
                                "id": 1,
                                "secondary_rack": null,
                                "mtu": 1500,
                                "primary_rack": "4y3h7n",
                                "name": "untagged",
                                "fabric": "fabric-0",
                                "dhcp_on": true,
                                "vid": 0
                            },
                            "dns_servers": [],
                            "space": "space-0",
                            "name": "192.168.100.0/24",
                            "gateway_ip": "192.168.100.1",
                            "cidr": "192.168.100.0/24"
                        }
                    }
                ]
            }
        ],
        "resource_uri": "/MAAS/api/2.0/machines/4y3ha4/",
        "hostname": "lowlier-glady",
        "status_name": "Ready",
        "min_hwe_kernel": "",
        "address_ttl": null,
        "boot_interface": {
            "effective_mtu": 1500,
            "mac_address": "52:54:00:33:6b:2c",
            "children": [],
            "discovered": [],
            "params": "",
            "vlan": {
                "resource_uri": "/MAAS/api/2.0/vlans/1/",
                "id": 1,
                "secondary_rack": null,
                "mtu": 1500,
                "primary_rack": "4y3h7n",
                "name": "untagged",
                "fabric": "fabric-0",
                "dhcp_on": true,
                "vid": 0
            },
            "name": "eth0",
            "enabled": true,
            "parents": [],
            "id": 39,
            "type": "physical",
            "resource_uri": "/MAAS/api/2.0/nodes/4y3ha4/interfaces/39/",
            "tags": [],
            "links": [
                {
                    "id": 67,
                    "mode": "auto",
                    "subnet": {
                        "resource_uri": "/MAAS/api/2.0/subnets/1/",
                        "id": 1,
                        "rdns_mode": 2,
                        "vlan": {
                            "resource_uri": "/MAAS/api/2.0/vlans/1/",
                            "id": 1,
                            "secondary_rack": null,
                            "mtu": 1500,
                            "primary_rack": "4y3h7n",
                            "name": "untagged",
                            "fabric": "fabric-0",
                            "dhcp_on": true,
                            "vid": 0
                        },
                        "dns_servers": [],
                        "space": "space-0",
                        "name": "192.168.100.0/24",
                        "gateway_ip": "192.168.100.1",
                        "cidr": "192.168.100.0/24"
                    }
                }
            ]
        },
        "power_state": "off",
        "architecture": "amd64/generic",
        "power_type": "virsh",
        "distro_series": "",
        "tag_names": [
            "virtual"
        ],
        "disable_ipv4": false,
        "status_message": "From 'Commissioning' to 'Ready'",
        "swap_size": null,
        "blockdevice_set": [
            {
                "path": "/dev/disk/by-dname/sda",
                "partition_table_type": "MBR",
                "name": "sda",
                "used_for": "MBR partitioned with 1 partition",
                "partitions": [
                    {
                        "bootable": false,
                        "id": 2,
                        "path": "/dev/disk/by-dname/sda-part1",
                        "filesystem": {
                            "fstype": "ext4",
                            "mount_point": "/",
                            "label": "root",
                            "mount_options": null,
                            "uuid": "7a0e75a8-0bc6-456b-ac92-4769e97baf02"
                        },
                        "type": "partition",
                        "resource_uri": "/MAAS/api/2.0/nodes/4y3ha4/blockdevices/35/partition/2",
                        "uuid": "6fe782cf-ad1a-4b31-8beb-333401b4d4bb",
                        "used_for": "ext4 formatted filesystem mounted at /",
                        "size": 8581545984
                    }
                ],
                "filesystem": null,
                "id_path": "/dev/disk/by-id/ata-QEMU_HARDDISK_QM00001",
                "resource_uri": "/MAAS/api/2.0/nodes/4y3ha4/blockdevices/35/",
                "id": 35,
                "serial": "QM00001",
                "block_size": 4096,
                "type": "physical",
                "used_size": 8586788864,
                "tags": [
                    "rotary"
                ],
                "available_size": 0,
                "uuid": null,
                "size": 8589934592,
                "model": "QEMU HARDDISK"
            }
        ],
        "zone": {
            "description": "",
            "resource_uri": "/MAAS/api/2.0/zones/default/",
            "name": "default"
        },
        "fqdn": "lowlier-glady.maas",
        "storage": 8589.934592,
        "node_type": 0,
        "boot_disk": null,
        "owner": null,
        "domain": {
            "id": 0,
            "name": "maas",
            "resource_uri": "/MAAS/api/2.0/domains/0/",
            "resource_record_count": 0,
            "ttl": null,
            "authoritative": true
        },
        "owner_data": {
            "braid": "jonathan blow",
            "frog-fractions": "jim crawford"
        }
    }
]
`
)
