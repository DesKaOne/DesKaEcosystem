package state

// PublicKeyResolver supplies the execution authority for a transaction sender.
// The execution layer owns the mapping; consensus and state do not persist it.
type PublicKeyResolver interface {
	PublicKeyForSender(sender []byte) ([]byte, error)
}
