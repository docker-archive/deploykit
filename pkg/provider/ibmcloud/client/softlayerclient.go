package client

import (
	"github.com/softlayer/softlayer-go/services"
	"github.com/softlayer/softlayer-go/session"
)

// SoftlayerClient for all SL API calls
type SoftlayerClient struct {
	sess    *session.Session
	account services.Account
}

// GetClient returns a SoftlayerClient instance
func GetClient(user, apiKey string) *SoftlayerClient {
	client := &SoftlayerClient{
		sess: session.New(user, apiKey),
	}
	client.account = services.GetAccountService(client.sess)
	return client
}

// AuthorizeToStorage authorizes a VM to a storage volume
func (c *SoftlayerClient) AuthorizeToStorage(storageID, guestID int) error {
	resType := "SoftLayer_Virtual_Guest"
	_, err := services.GetNetworkStorageService(c.sess).Id(storageID).AllowAccessFromHost(&resType, &guestID)
	return err
}

// DeauthorizeFromStorage removes the VM authorization for a storage volume
func (c *SoftlayerClient) DeauthorizeFromStorage(storageID, guestID int) error {
	resType := "SoftLayer_Virtual_Guest"
	_, err := services.GetNetworkStorageService(c.sess).Id(storageID).RemoveAccessFromHost(&resType, &guestID)
	return err
}

// GetAllowedStorageVirtualGuests gets all VM IDs that are authorized to the storage volume
func (c *SoftlayerClient) GetAllowedStorageVirtualGuests(storageID int) ([]int, error) {
	resp, err := services.GetNetworkStorageService(c.sess).Id(storageID).GetAllowedVirtualGuests()
	if err != nil {
		return []int{}, err
	}
	result := []int{}
	for _, r := range resp {
		result = append(result, *r.Id)
	}
	return result, nil
}
