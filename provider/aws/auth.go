package aws

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
)

// TODO(chungers) - This is copied from libmachete.

// Credential defines static authentication details (as opposed to environment-based) for the AWS API.
type Credential struct {
	AccessKeyID     string `yaml:"access_key_id" json:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key" json:"secret_access_key"`
	SessionToken    string `yaml:"session_token" json:"session_token"`
}

// Retrieve implements the AWS credentials.Provider interface method
func (a *Credential) Retrieve() (credentials.Value, error) {
	return credentials.Value{
		AccessKeyID:     a.AccessKeyID,
		SecretAccessKey: a.SecretAccessKey,
		SessionToken:    a.SessionToken,
		ProviderName:    "Machete",
	}, nil
}

// IsExpired implements the AWS credentials.Provider interface method.  For static credentials this always returns false
func (a *Credential) IsExpired() bool {
	return false
}
