package aws

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/machines"
	"github.com/docker/libmachete/provisioners/spi"
)

// importEC2Key imports a generated SSH key to EC2.
// It also mutates the input request to use the generated key.
func (p provisioner) importEC2Key(resource spi.Resource, request spi.MachineRequest, events chan<- interface{}) error {
	createInstanceRequest, err := ensureRequestType(request)
	if err != nil {
		return err
	}

	keyName := resource.Name()
	publicKey, err := p.sshKeys.GetEncodedPublicKey(api.SSHKeyID(keyName))
	if err != nil {
		events <- err
		return err
	}

	// AWS requires that the key be uploaded prior to creating the instance
	_, err = p.client.ImportKeyPair(&ec2.ImportKeyPairInput{KeyName: &keyName, PublicKeyMaterial: publicKey})
	if err != nil {
		return err
	}

	// Now we have successfully imported the key, change the input to use this
	createInstanceRequest.KeyName = keyName

	// Send this change to be logged.
	events <- createInstanceRequest
	return nil
}

// deleteEC2Key removes an imported key from EC2.
func (p provisioner) deleteEC2Key(
	resource spi.Resource,
	request spi.MachineRequest,
	events chan<- interface{}) error {

	keyName := resource.Name()

	// AWS requires that the key be uploaded prior to creating the instance
	_, err := p.client.DeleteKeyPair(&ec2.DeleteKeyPairInput{KeyName: &keyName})
	return err
}
