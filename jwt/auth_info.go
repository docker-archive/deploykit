package jwt

import (
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// BearerJWTToken provides a header based jwt access token auth info writer
func BearerJWTToken(token string) runtime.ClientAuthInfoWriter {
	return runtime.ClientAuthInfoWriterFunc(func(r runtime.ClientRequest, _ strfmt.Registry) error {
		r.SetHeaderParam("Authorization", "JWT "+token)
		return nil
	})
}
