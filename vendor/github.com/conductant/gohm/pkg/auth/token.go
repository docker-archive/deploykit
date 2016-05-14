package auth

import (
	"crypto/rsa"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"net/http"
	"time"
)

// JWT token that uses RSA public/private key pairs for signing and verification.
// The signer uses a RSA private key to sign while the receiver verifies the key
// using the public key to verify the signature.
type Token struct {
	token *jwt.Token
}

var (
	EmptyExpiringToken = NewToken(0 * time.Second)
)

func NewToken(ttl time.Duration) *Token {
	token := &Token{token: jwt.New(jwt.SigningMethodRS512)}
	token.SetExpiration(ttl)
	return token
}

func (this *Token) SignedString(key func() []byte) (string, error) {
	if privateKey, err := RsaPrivateKeyFromPem(key); err != nil {
		return "", err
	} else {
		return this.token.SignedString(privateKey)
	}
}

func (this *Token) SetHeader(header http.Header, key func() []byte) error {
	if key == nil {
		return ErrNoPrivateKeyFunc
	}
	if sk, err := this.SignedString(key); err == nil {
		header.Set("Authorization", "Bearer "+sk)
		return nil
	} else {
		return err
	}
}

func checkAlg(t *jwt.Token) error {
	if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
		return fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
	}
	return nil
}

func RsaPrivateKeyFromPem(source func() []byte) (*rsa.PrivateKey, error) {
	return jwt.ParseRSAPrivateKeyFromPEM(source())
}

func RsaPublicKeyFromPem(source func() []byte) (func(*jwt.Token) (interface{}, error), error) {
	if key, err := jwt.ParseRSAPublicKeyFromPEM(source()); err != nil {
		return nil, err
	} else {
		return func(t *jwt.Token) (interface{}, error) {
			if err := checkAlg(t); err == nil {
				return key, nil
			} else {
				return nil, err
			}
		}, nil
	}
}

func TokenFromString(tokenString string, key func() []byte, now func() time.Time) (*Token, error) {
	if key == nil {
		return nil, ErrNoPublicKeyFunc
	}

	if keyFunc, err := RsaPublicKeyFromPem(key); err != nil {
		return nil, err
	} else {
		if t, err := jwt.Parse(tokenString, keyFunc); err != nil {
			return nil, err
		} else {
			return checkTokenExpiration(t, now)
		}
	}
}

// parses from header or query
// https://github.com/dgrijalva/jwt-go/blob/master/token.go#L94
// Either Authorization: Bearer ....
// Or query parameter 'access_token=...'
func TokenFromHttp(req *http.Request, key func() []byte, now func() time.Time) (*Token, error) {
	if key == nil {
		return nil, ErrNoPublicKeyFunc
	}

	if keyFunc, err := RsaPublicKeyFromPem(key); err != nil {
		return nil, err
	} else {
		if t, err := jwt.ParseFromRequest(req, keyFunc); err != nil {
			if err == jwt.ErrNoTokenInRequest {
				return nil, ErrNoAuthToken
			} else {
				return nil, err
			}
		} else {
			return checkTokenExpiration(t, now)
		}
	}
}

func (this *Token) SetExpiration(d time.Duration) {
	if d > 0 {
		this.token.Claims["exp"] = time.Now().Add(d).Unix()
	}
}

func (this *Token) Add(key string, value interface{}) *Token {
	this.token.Claims[key] = value
	return this
}

func (this *Token) Get(key string) interface{} {
	if v, has := this.token.Claims[key]; has {
		return v
	}
	return nil
}

func (this *Token) GetString(key string) string {
	if v := this.Get(key); v == nil {
		return ""
	} else {
		return fmt.Sprintf("%s", v)
	}
}

func (this *Token) HasKey(key string) bool {
	if _, has := this.token.Claims[key]; has {
		return true
	}
	return false
}

func checkTokenExpiration(t *jwt.Token, now func() time.Time) (*Token, error) {
	if t == nil || !t.Valid {
		return nil, ErrInvalidAuthToken
	}
	// Check expiration if there is one
	if expClaim, has := t.Claims["exp"]; has {
		exp, ok := expClaim.(float64)
		if !ok {
			return nil, ErrInvalidAuthToken
		}
		if now().After(time.Unix(int64(exp), 0)) {
			return nil, ErrExpiredAuthToken
		}
	}
	return &Token{token: t}, nil
}
