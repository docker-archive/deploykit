package storage

// MachineID is the globally-unique identifier for machines.
type MachineID string

// Machines handles storage of machine inventory.  In addition to standard fields for all machines,
// it allows provisioners to include custom data.
type Machines interface {
	Save(record MachineRecord, provisionerData interface{}) error

	List() ([]MachineID, error)

	GetRecord(id MachineID) (*MachineRecord, error)

	GetDetails(id MachineID, provisionerData interface{}) error

	Delete(id MachineID) error
}

// Timestamp is a unix epoch timestamp, in seconds.
type Timestamp uint64

// MachineRecord is the storage structure that will be included for all machines.
type MachineRecord struct {
	Name         MachineID
	Provisioner  string
	Created      Timestamp
	LastModified Timestamp
}

// CredentialsID is the globally-unique identifier for credentials.
type CredentialsID string

// Credentials handles storage of identities and secrets for authenticating with third-party
// systems.
type Credentials interface {
	Save(id CredentialsID, credentialsData interface{}) error

	List() ([]CredentialsID, error)

	GetCredentials(id CredentialsID, credentialsData interface{}) error

	Delete(id CredentialsID) error
}
