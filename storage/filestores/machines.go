package filestores

import (
	"fmt"
	"github.com/docker/libmachete/storage"
	"path"
)

type machines struct {
	sandbox Sandbox
}

// NewMachines creates a new machine store within the provided sandbox.
func NewMachines(sandbox Sandbox) storage.Machines {
	return &machines{sandbox: sandbox}
}

// Save saves the record and detail.  The detail can be nil if no new state is known
func (m machines) Save(record storage.MachineRecord, provisionerData interface{}) error {
	err := m.sandbox.mkdir(string(record.MachineName))
	if err != nil {
		return fmt.Errorf("Failed to create machine directory: %s", err)
	}

	err = m.sandbox.marshalAndSave(m.recordPath(record.MachineName), record)
	if err != nil {
		return err
	}

	if provisionerData != nil {
		err = m.sandbox.marshalAndSave(m.provisionerRecordPath(record.MachineName), provisionerData)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m machines) List() ([]storage.MachineID, error) {
	contents, err := m.sandbox.list()
	if err != nil {
		return nil, err
	}
	ids := []storage.MachineID{}
	for _, element := range contents {
		ids = append(ids, storage.MachineID(element))
	}
	return ids, nil
}

func (m machines) GetRecord(id storage.MachineID) (*storage.MachineRecord, error) {
	record := new(storage.MachineRecord)
	err := m.sandbox.readAndUnmarshal(m.recordPath(id), record)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (m machines) GetDetails(id storage.MachineID, provisionerData interface{}) error {
	err := m.sandbox.readAndUnmarshal(m.provisionerRecordPath(id), provisionerData)
	return err
}

func (m machines) Delete(id storage.MachineID) error {
	return m.sandbox.removeAll(m.machinePath(id))
}

func (m machines) machinePath(id storage.MachineID) string {
	return string(id)
}

func (m machines) recordPath(id storage.MachineID) string {
	return path.Join(m.machinePath(id), "machine.json")
}

func (m machines) provisionerRecordPath(id storage.MachineID) string {
	return path.Join(m.machinePath(id), "details.json")
}
