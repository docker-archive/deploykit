package main

import (
	"fmt"
	"net/url"

	"github.com/FrenchBen/oracle-sdk-go/bmc"
	"github.com/FrenchBen/oracle-sdk-go/compute"
)

func main() {
	apiEndpoint, err := url.Parse("myAPIEndpoint")
	if err != nil {
		fmt.Errorf("Error parsing API Endpoint: %s", err)
	}

	config := &bmc.Config{
		User:        bmc.String("ocid1.user.oc1..aaaaaaaat5nvwcna5j6aqzjcaty5eqbb6qt2jvpkanghtgdaqedqw3rynjq"),
		Fingerprint: bmc.String("20:3b:97:13:55:1c:5b:0d:d3:37:d8:50:4e:c5:3a:34"),
		KeyFile:     bmc.String("~/.oraclebmc/bmc_api_key.pem"),
		APIEndpoint: apiEndpoint,
		LogLevel:    bmc.LogLevel(bmc.LogDebug),
		// Logger: # Leave blank to use the default logger, or provide your own
		// HTTPClient: # Leave blank to use default HTTP Client, or provider your own
	}
	// or
	// config := bmc.FromConfigFile()

	// Create the Compute Client
	client, err := compute.NewComputeClient(config)
	if err != nil {
		fmt.Errorf("Error creating OPC Compute Client: %s", err)
	}
	// Create instances client
	instanceClient := client.Instances()

	// Instances Input
	input := &compute.CreateInstanceInput{
		Name:       "test-instance",
		Label:      "test",
		Shape:      "oc3",
		ImageList:  "/oracle/public/oel_6.7_apaas_16.4.5_1610211300",
		Storage:    nil,
		BootOrder:  nil,
		SSHKeys:    []string{},
		Attributes: map[string]interface{}{},
	}

	// Create the instance
	instance, err := instanceClient.CreateInstance(input)
	if err != nil {
		fmt.Errorf("Error creating instance: %s", err)
	}
	fmt.Printf("Instance Created: %#v", instance)
}
