package compute

import (
	"fmt"
	"testing"
)

var createdInstanceName *InstanceName

func TestInstanceLifecycle(t *testing.T) {
	defer tearDownInstances()

	svc, err := getInstancesClient()
	if err != nil {
		t.Fatal(err)
	}

	createdInstanceName, err = svc.LaunchInstance("test", "test", "oc3", "/oracle/public/oel_6.4_2GB_v1", nil, nil, []string{},
		map[string]interface{}{
			"attr1": 12,
			"attr2": map[string]interface{}{
				"inner_attr1": "foo",
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Instance created: %#v\n", createdInstanceName)

	instanceInfo, err := svc.WaitForInstanceRunning(createdInstanceName, 120)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Instance retrieved: %#v\n", instanceInfo)
}

func tearDownInstances() {
	svc, err := getInstancesClient()
	if err != nil {
		panic(err)
	}

	err = svc.DeleteInstance(createdInstanceName)
	if err != nil {
		panic(err)
	}

	err = svc.WaitForInstanceDeleted(createdInstanceName, 600)
	if err != nil {
		panic(err)
	}
}

func getInstancesClient() (*InstancesClient, error) {
	authenticatedClient, err := getAuthenticatedClient()
	if err != nil {
		return &InstancesClient{}, err
	}

	return authenticatedClient.Instances(), nil
}
