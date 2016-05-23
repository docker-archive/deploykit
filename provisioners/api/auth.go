package api

import (
	"fmt"
	"golang.org/x/net/context"
)

const (
	// Version is the current version of the API -- TODO - use compile time linker flags to set this value from git tag.
	Version = "TODO"
)

// CredentialBase is a common base struct that should be reused by provisioner via embedding.
type CredentialBase struct {
	Provisioner string `yaml:"provisioner" json:"provisioner"`
}

// ProvisionerName returns the name of the provisioner this credentail object is for.
func (cb CredentialBase) ProvisionerName() string {
	return cb.Provisioner
}

// Validate validates the credential based on some runtime context.  For example, is
// the credential for a given region or Oauth client id.
func (cb CredentialBase) Validate(context.Context) error {
	if cb.Provisioner == "" {
		return fmt.Errorf("no provisioner")
	}
	return nil
}

// Authenticate authenticates the user modeled by this credential against the backend
func (cb CredentialBase) Authenticate(context.Context) error {
	return fmt.Errorf("base not-implemented")
}

// Refresh refreshes the credential, if it makes sense for a given provisioner
func (cb *CredentialBase) Refresh(context.Context) error {
	return fmt.Errorf("base not-implemented")
}

// Credential is the interface that all provisioners requiring auth must provide implementation for.
// For example, a struct containing access id and secret key that can be loaded or saved from storage or
// user input provides the method implementations to validate, authenticate, and optionally refresh itself.
type Credential interface {
	ProvisionerName() string

	Authenticate(context.Context) error

	Refresh(context.Context) error
}

// Auth is an interface each provisioner is exepcted to implement.
// The provisioners are free to define their own structures and return an instance when asked.
// The responsibility of populating the structure is with the caller which will unmarshal on disk
// files or CLI flags or HTTP requests.
type Auth interface {
	NewCredential() Credential
}
