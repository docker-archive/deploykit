package storage

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/storage"
)

// Key is the globally-unique identifier for machines.
type Key string

// Machines handles storage of machine inventory.  In addition to standard fields for all machines,
// it allows provisioners to include custom data.
type Machines interface {
	Save(record Record, provisionerData interface{}) error

	List() ([]*Record, error)

	GetRecord(key Key) (*Record, error)

	GetDetails(key Key, provisionerData interface{}) error

	Delete(key Key) error
}

// Timestamp is a unix epoch timestamp, in seconds.
type Timestamp uint32

// Record is the storage structure that will be included for all machines.
type Record struct {
	Name         string
	Provisioner  string
	Created      Timestamp
	LastModified Timestamp
}

type machineStore struct {
	store storage.Storage
}

// New creates a Machines instance using the supplied storage backend.
func New(store storage.Storage) Machines {
	return &machineStore{store: store}
}

type compositeRecord struct {
	StoredRecord          Record
	JSONProvisionerRecord []byte
}

func (m machineStore) Save(record Record, provisionerRecord interface{}) error {
	// TODO(wfarner): Populate Record timestamps, etc.

	provisionerData, err := json.Marshal(provisionerRecord)
	if err != nil {
		return err
	}

	composite := compositeRecord{
		StoredRecord:          record,
		JSONProvisionerRecord: provisionerData,
	}

	data, err := json.Marshal(composite)
	if err != nil {
		return err
	}

	return m.store.Write(record.Name, data)
}

func extractRecord(id string, data []byte, provisionerRecord interface{}) (*Record, error) {
	composite := new(compositeRecord)
	err := json.Unmarshal(data, composite)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode internal record %s: %s", id, err)
	}

	if provisionerRecord != nil {
		err = json.Unmarshal(composite.JSONProvisionerRecord, provisionerRecord)
		if err != nil {
			return nil, fmt.Errorf("Failed to decode provisioner record %s: %s", id, err)
		}
	}

	return &composite.StoredRecord, nil
}

func (m machineStore) List() ([]*Record, error) {
	entries, err := m.store.ReadAll()
	if err != nil {
		return nil, err
	}

	records := []*Record{}
	for id, encodedRecord := range entries {
		record, err := extractRecord(id, encodedRecord, nil)
		if err != nil {
			return nil, err
		}

		records = append(records, record)
	}

	return records, nil
}

func (m machineStore) getCompositeRecord(key Key, provisionerData interface{}) (*Record, error) {
	keyString := string(key)
	value, err := m.store.Read(keyString)
	if err != nil {
		return nil, err
	}

	return extractRecord(keyString, value, provisionerData)
}

func (m machineStore) GetRecord(key Key) (*Record, error) {
	return m.getCompositeRecord(key, nil)
}

func (m machineStore) GetDetails(key Key, provisionerData interface{}) error {
	_, err := m.getCompositeRecord(key, provisionerData)
	return err
}

func (m machineStore) Delete(key Key) error {
	return m.store.Delete(string(key))
}
