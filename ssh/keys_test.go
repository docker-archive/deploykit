package ssh

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestNewKeyPair(t *testing.T) {

	kp, err := NewKeyPair()
	require.NoError(t, err)
	require.NotNil(t, kp)

	require.True(t, len(kp.PublicKey) > 0)
	require.True(t, len(kp.PrivateKey) > 0)
	require.True(t, len(kp.EncodedPublicKey) > 0)
	require.True(t, len(kp.EncodedPrivateKey) > 0)
	require.True(t, len(kp.Fingerprint()) > 0)

	buff, err := json.MarshalIndent(kp, " ", " ")
	require.NoError(t, err)

	t.Log(string(buff))

	kp2 := new(KeyPair)
	err = json.Unmarshal(buff, kp2)
	require.NoError(t, err)

	require.Equal(t, 0, strings.Index(string(kp.EncodedPrivateKey), "-----BEGIN RSA PRIVATE KEY-----"))
	require.Equal(t, 0, strings.Index(string(kp.EncodedPublicKey), "ssh-rsa"))
}
