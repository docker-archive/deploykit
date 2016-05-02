package storage

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/storage"
)

// Machines handles storage of machine inventory.  In addition to standard fields for all machines,
// it allows provisioners to include custom data.
type Machines interface {
	Save(record Record, provisionerData interface{}) error
	List() ([]*Record, error)
	Get(name string, recordType interface{}) (*Record, error)
	Delete(name string) error
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

func (m machineStore) Get(id string, provisionerRecord interface{}) (*Record, error) {
	value, err := m.store.Read(id)
	if err != nil {
		return nil, err
	}

	return extractRecord(id, value, provisionerRecord)
}

func (m machineStore) Delete(id string) error {
	return m.store.Delete(id)
}
