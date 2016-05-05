package filestores

import (
	"fmt"
	"github.com/docker/libmachete/storage"
	"path"
)

type machines struct {
	sandbox *sandbox
}

// NewMachines creates a new machine store backed by the local file system.
func NewMachines(dir string) (storage.Machines, error) {
	sandbox, err := newSandbox(dir)
	if err != nil {
		return nil, err
	}

	return &machines{sandbox: sandbox}, nil
}

func (m machines) Save(record storage.MachineRecord, provisionerData interface{}) error {
	err := m.sandbox.Mkdir(string(record.Name))
	if err != nil {
		return fmt.Errorf("Failed to create machine directory: %s", err)
	}

	err = m.sandbox.MarshalAndSave(m.recordPath(record.Name), record)
	if err != nil {
		return err
	}

	err = m.sandbox.MarshalAndSave(m.provisionerRecordPath(record.Name), provisionerData)
	if err != nil {
		return err
	}

	return nil
}

func (m machines) List() ([]storage.MachineID, error) {
	contents, err := m.sandbox.List()
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
	err := m.sandbox.ReadAndUnmarshal(m.recordPath(id), record)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (m machines) GetDetails(id storage.MachineID, provisionerData interface{}) error {
	err := m.sandbox.ReadAndUnmarshal(m.provisionerRecordPath(id), provisionerData)
	return err
}

func (m machines) Delete(id storage.MachineID) error {
	return m.sandbox.RemoveAll(m.machinePath(id))
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
