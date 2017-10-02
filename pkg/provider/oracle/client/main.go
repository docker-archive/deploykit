package main

import (
	"fmt"
	"log"

	"github.com/docker/infrakit/pkg/provider/oracle/client/api"
	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"
	"github.com/docker/infrakit/pkg/provider/oracle/client/core"
)

func main() {
	config := &bmc.Config{
		User:        bmc.String("ocid1.user.oc1..aaaaaaaat5nvwcna5j6aqzjcaty5eqbb6qt2jvpkanghtgdaqedqw3rynjq"),
		Fingerprint: bmc.String("20:3b:97:13:55:1c:5b:0d:d3:37:d8:50:4e:c5:3a:34"),
		KeyFile:     bmc.String("bmc_api_key.pem"),
		Region:      bmc.String("us-phoenix-1"),
		LogLevel:    bmc.LogLevel(bmc.LogDebug),
	}

	// Create the Compute Client
	client, err := api.NewClient(config)
	if err != nil {
		log.Fatal("Error creating OPC Compute Client: ", err)
	}

	// Create instances client for the CompartmentID desired
	coreClient := core.NewClient(client, "ocid1.compartment.oc1..aaaaaaaam3we6vgnherjq5q2idnccdflvjsnog7mlr6rtdb25gilchfeyjxa")
	options := &core.InstancesParameters{
		Limit: 500,
		Page:  "1",
		Filter: &core.InstanceFilter{
			DisplayName:    "MyInst*", // Filter instance names using globbing
			LifeCycleState: "RUNNING", // and by state (uppercase and lowercase accepted)
		},
	}
	instances, bmcErr := coreClient.ListInstances(options)
	if bmcErr != nil {
		log.Fatal("Error listing compute instance: ", bmcErr.Error())
	}
	fmt.Println("Instances: ", instances)
}
