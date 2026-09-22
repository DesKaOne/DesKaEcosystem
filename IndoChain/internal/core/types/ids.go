package types

// Address is a protocol-level account/address identifier.
// Its byte representation remains subject to the crypto/address specification.
type Address []byte

func (a Address) Bytes() []byte {
 out := make([]byte, len(a))
 copy(out, a)
 return out
}