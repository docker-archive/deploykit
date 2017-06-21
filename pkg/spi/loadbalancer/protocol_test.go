package loadbalancer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProtocolParse(t *testing.T) {
	require.Equal(t, HTTP, ProtocolFromString("HTTP"))
	require.Equal(t, HTTP, ProtocolFromString("http"))
	require.Equal(t, HTTPS, ProtocolFromString("HTTPS"))
	require.Equal(t, HTTPS, ProtocolFromString("https"))
	require.Equal(t, TCP, ProtocolFromString("TCP"))
	require.Equal(t, TCP, ProtocolFromString("tcp"))
	require.Equal(t, SSL, ProtocolFromString("SSL"))
	require.Equal(t, SSL, ProtocolFromString("ssl"))
	require.Equal(t, Invalid, ProtocolFromString("bogus"))
}
