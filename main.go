package main

import (
	"fmt"

	"github.com/FrenchBen/oracle-sdk-go/bmc"
	"github.com/FrenchBen/oracle-sdk-go/compute"
)

func main() {
	config := &bmc.Config{
		User:        bmc.String("ocid1.user.oc1..aaaaaaaat5nvwcna5j6aqzjcaty5eqbb6qt2jvpkanghtgdaqedqw3rynjq"),
		Fingerprint: bmc.String("20:3b:97:13:55:1c:5b:0d:d3:37:d8:50:4e:c5:3a:34"),
		KeyFile:     bmc.String("bmc_api_key.pem"),
		APIEndpoint: bmc.GetAPIEndpoint("us-phoenix-1"),
		LogLevel:    bmc.LogLevel(bmc.LogDebug),
	}

	// Create the Compute Client
	client, err := compute.NewComputeClient(config)
	if err != nil {
		fmt.Errorf("Error creating OPC Compute Client: %s", err)
	}

	// Create instances client for the CompartmentID desired
	instanceClient := client.NewInstanceClient("ocid1.compartment.oc1..aaaaaaaam3we6vgnherjq5q2idnccdflvjsnog7mlr6rtdb25gilchfeyjxa")
	options := &compute.InstancesParameters{
		Limit: 500,
		Page:  "1",
	}
	instanceClient.ListInstances(options)
}
