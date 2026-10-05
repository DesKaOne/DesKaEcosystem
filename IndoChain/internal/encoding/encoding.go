package encoding

// Codec defines the canonical serialization boundary for protocol objects.
// The concrete encoding remains intentionally unspecified until protocol freeze.
type Codec interface {
	Encode(value any) ([]byte, error)
	Decode(data []byte, value any) error
}
