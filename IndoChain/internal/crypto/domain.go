package crypto

import "encoding/binary"

// DomainSeparatedMessage prefixes a protocol domain and version before signing.
func DomainSeparatedMessage(domain string, version uint16, message []byte) []byte {
	out := make([]byte, 4+len(domain)+len(message))
	binary.BigEndian.PutUint16(out[:2], version)
	binary.BigEndian.PutUint16(out[2:4], uint16(len(domain)))
	copy(out[4:4+len(domain)], domain)
	copy(out[4+len(domain):], message)
	return out
}
