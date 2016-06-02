package aws

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/docker/libmachete/provisioners/api"
	"golang.org/x/net/context"
)

// NewCredential allocates a blank credential object.
func NewCredential() api.Credential {
	return &credential{CredentialBase: api.CredentialBase{}}
}

type credential struct {
	api.CredentialBase `yaml:",inline"`
	AccessKeyID        string `yaml:"access_key_id" json:"access_key_id"`
	SecretAccessKey    string `yaml:"secret_access_key" json:"secret_access_key"`
	SessionToken       string `yaml:"session_token" json:"session_token"`
}

// Retrieve implements the AWS credentials.Provider interface method
func (a *credential) Retrieve() (credentials.Value, error) {
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

// Authenticate implements Credential interface method
func (a *credential) Authenticate(ctx context.Context) error {
	return nil
}

// Refresh implements Credential interface method
func (a *credential) Refresh(ctx context.Context) error {
	return nil
}
