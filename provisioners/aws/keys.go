package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/provisioners/api"
)

// GenerateAndUploadSSHKey overrides the default SSHKeyGen task to first generate the key then upload to EC2.
// It also mutates the input request to use the generated key.
func GenerateAndUploadSSHKey(
	p api.Provisioner,
	keystore api.KeyStore,
	cred api.Credential,
	resource api.Resource,
	request api.MachineRequest, events chan<- interface{}) error {

	ci, err := ensureRequestType(request)
	if err != nil {
		return err
	}

	prov, is := p.(*provisioner)

	if !is {
		return fmt.Errorf("Not AWS provisioner:%v", p)
	}

	// Call the default implementation to generate the key
	if err := libmachete.TaskSSHKeyGen.Do(prov, keystore, cred, resource, request, events); err != nil {
		return err
	}

	keyName := resource.Name()
	publicKey, err := keystore.GetEncodedPublicKey(keyName)
	if err != nil {
		events <- err
		return err
	}

	// AWS requires that the key be uploaded prior to creating the instance
	if _, err = prov.client.ImportKeyPair(&ec2.ImportKeyPairInput{
		KeyName:           &keyName,
		PublicKeyMaterial: publicKey,
	}); err != nil {
		return err
	}

	// Now we have successfully imported the key, change the input to use this
	ci.KeyName = keyName

	// Send this change to be logged.
	events <- ci
	return nil
}

// RemoveLocalAndUploadedSSHKey removes the local ssh key and calls EC2 api to remove the uploaded key
func RemoveLocalAndUploadedSSHKey(
	p api.Provisioner,
	keystore api.KeyStore,
	cred api.Credential,
	resource api.Resource,
	request api.MachineRequest,
	events chan<- interface{}) error {

	prov, is := p.(*provisioner)

	if !is {
		return fmt.Errorf("Not AWS provisioner:%v", p)
	}

	keyName := resource.Name()

	// AWS requires that the key be uploaded prior to creating the instance
	if _, err := prov.client.DeleteKeyPair(&ec2.DeleteKeyPairInput{
		KeyName: &keyName,
	}); err != nil {
		return err
	}

	// Call the default implementation to generate the key
	if err := libmachete.TaskSSHKeyRemove.Do(prov, keystore, cred, resource, request, events); err != nil {
		return err
	}

	return nil
}
