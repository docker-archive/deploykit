package client

import (
	"sync"

	"github.com/softlayer/softlayer-go/datatypes"
)

// FakeSoftlayer is used for testing
type FakeSoftlayer struct {
	GetVirtualGuestsStub  func(mask, filters *string) ([]datatypes.Virtual_Guest, error)
	getVirtualGuestsMutex sync.RWMutex
	GetVirtualGuestsArgs  []struct {
		Mask    *string
		Filters *string
	}

	AuthorizeToStorageStub  func(storageID, guestID int) error
	authorizeToStorageMutex sync.RWMutex
	AuthorizeToStorageArgs  []struct {
		StorageID int
		GuestID   int
	}

	DeauthorizeFromStorageStub  func(storageID, guestID int) error
	deauthorizeFromStorageMutex sync.RWMutex
	DeauthorizeFromStorageArgs  []struct {
		StorageID int
		GuestID   int
	}

	GetAllowedStorageVirtualGuestsStub  func(storageID int) ([]int, error)
	getAllowedStorageVirtualGuestsMutex sync.RWMutex
	GetAllowedStorageVirtualGuestsArgs  []struct {
		StorageID int
	}
}

// GetVirtualGuests .
func (fake *FakeSoftlayer) GetVirtualGuests(mask, filters *string) ([]datatypes.Virtual_Guest, error) {
	fake.getVirtualGuestsMutex.Lock()
	defer fake.getVirtualGuestsMutex.Unlock()
	fake.GetVirtualGuestsArgs = append(fake.GetVirtualGuestsArgs, struct {
		Mask    *string
		Filters *string
	}{mask, filters})
	return fake.GetVirtualGuestsStub(mask, filters)
}

// AuthorizeToStorage .
func (fake *FakeSoftlayer) AuthorizeToStorage(storageID, guestID int) error {
	fake.authorizeToStorageMutex.Lock()
	defer fake.authorizeToStorageMutex.Unlock()
	fake.AuthorizeToStorageArgs = append(fake.AuthorizeToStorageArgs, struct {
		StorageID int
		GuestID   int
	}{storageID, guestID})
	return fake.AuthorizeToStorageStub(storageID, guestID)
}

// DeauthorizeFromStorage .
func (fake *FakeSoftlayer) DeauthorizeFromStorage(storageID, guestID int) error {
	fake.deauthorizeFromStorageMutex.Lock()
	defer fake.deauthorizeFromStorageMutex.Unlock()
	fake.DeauthorizeFromStorageArgs = append(fake.DeauthorizeFromStorageArgs, struct {
		StorageID int
		GuestID   int
	}{storageID, guestID})
	return fake.DeauthorizeFromStorageStub(storageID, guestID)
}

// GetAllowedStorageVirtualGuests .
func (fake *FakeSoftlayer) GetAllowedStorageVirtualGuests(storageID int) ([]int, error) {
	fake.getAllowedStorageVirtualGuestsMutex.Lock()
	defer fake.getAllowedStorageVirtualGuestsMutex.Unlock()
	fake.GetAllowedStorageVirtualGuestsArgs = append(fake.GetAllowedStorageVirtualGuestsArgs, struct {
		StorageID int
	}{storageID})
	return fake.GetAllowedStorageVirtualGuestsStub(storageID)
}
