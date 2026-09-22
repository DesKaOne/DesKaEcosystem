package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"sort"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

// Root returns a deterministic development commitment over the complete account
// state. The algorithm is intentionally not the final protocol StateRoot.
func (s *State) Root() types.Hash {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.accounts))
	for key := range s.accounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b bytes.Buffer
	for _, key := range keys {
		account := s.accounts[key]
		writeU32(&b, uint32(len(key)))
		_, _ = b.WriteString(key)
		writeU64(&b, account.Balance)
		writeU64(&b, uint64(account.Nonce))
	}

	return sha256.Sum256(b.Bytes())
}

func writeU32(b *bytes.Buffer, v uint32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	_, _ = b.Write(buf[:])
}

func writeU64(b *bytes.Buffer, v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	_, _ = b.Write(buf[:])
}
