package aws

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/docker/libmachete/provisioners/api"
	"golang.org/x/net/context"
)

const (
	// ProvisionerName is a unique name for this provisioner.
	// It is used in all API / CLI to identify the provisioner.
	ProvisionerName = "aws"
)

// NewCredential allocates a blank credential object.  Calling Validate() on this object will result in error.
func NewCredential() api.Credential {
	return &credential{
		Provisioner: ProvisionerName,
	}
}

type credential struct {
	Provisioner     string `yaml:"provisioner" json:"provisioner"`
	AccessKeyID     string `yaml:"access_key_id" json:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key" json:"secret_access_key"`
	SessionToken    string `yaml:"session_token" json:"session_token"`
}

// Retrieve implements the AWS credentials.Provider interface method
func (a *credential) Retrieve() (credentials.Value, error) {

	if err := a.Validate(nil); err != nil {
		return credentials.Value{}, err
	}

	return credentials.Value{
		AccessKeyID:     a.AccessKeyID,
		SecretAccessKey: a.SecretAccessKey,
		SessionToken:    a.SessionToken,
		ProviderName:    "Machete",
	}, nil
}

// IsExpired implements the AWS credentials.Provider interface method.  For static credentials this always returns false
func (a *credential) IsExpired() bool {
	return false
}

// ProvisionerName implements Credential interface method
func (a credential) ProvisionerName() string {
	return a.Provisioner
}

// Validate implements Credential interface method
func (a credential) Validate(ctx context.Context) error {
	if a.AccessKeyID == "" || a.SecretAccessKey == "" {
		return credentials.ErrStaticCredentialsEmpty
	}
	return nil
}

// Authenticate implements Credential interface method
func (a *credential) Authenticate(ctx context.Context) error {
	return nil
}

// Refresh implements Credential interface method
func (a *credential) Refresh(ctx context.Context) error {
	return nil
}
