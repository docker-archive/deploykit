package loadbalancer

import (
	"strings"
)

// Protocol is a network protocol.
type Protocol string

var (
	// HTTP -  HTTP
	HTTP = Protocol("HTTP")

	// HTTPS - HTTPS
	HTTPS = Protocol("HTTPS")

	// TCP - TCP
	TCP = Protocol("TCP")

	// SSL -  SSL
	SSL = Protocol("SSL")

	// UDP - UDP
	UDP = Protocol("UDP")

	// Invalid - This is the 'nil' value
	Invalid = Protocol("")
)

// ProtocolFromString gets the matching protocol for a string value.
func ProtocolFromString(protocol string) Protocol {
	for _, p := range []Protocol{HTTP, HTTPS, TCP, SSL, UDP} {
		if string(p) == strings.ToUpper(protocol) {
			return p
		}
	}
	return Invalid
}

// Valid tests whether a protocol is known and valid.
func (p *Protocol) Valid() bool {
	return ProtocolFromString(string(*p)) != Invalid
}
