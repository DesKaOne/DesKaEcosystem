package devnet

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"sort"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

const (
	ChainID         types.ChainID         = 1001
	ProtocolVersion types.ProtocolVersion = 1
	NetworkProfile                         = "devnet"
	Timestamp       int64                 = 1767225600 // 2026-01-01T00:00:00Z
)

// Genesis is the development genesis configuration.
// Its encoding and parameter schema remain subject to protocol freeze.
type Genesis struct {
	ChainID            types.ChainID
	NetworkProfile     string
	ProtocolVersion    types.ProtocolVersion
	Timestamp          int64
	InitialValidators  [][]byte
	InitialAccounts    [][]byte
	InitialAllocations map[string]uint64
	ProtocolParameters map[string]uint64
}

// Default returns the deterministic Devnet genesis configuration.
func Default() Genesis {
	return Genesis{
		ChainID:            ChainID,
		NetworkProfile:     NetworkProfile,
		ProtocolVersion:    ProtocolVersion,
		Timestamp:          Timestamp,
		InitialValidators:  nil,
		InitialAccounts:    nil,
		InitialAllocations: map[string]uint64{},
		ProtocolParameters: map[string]uint64{},
	}
}

// State builds the deterministic initial account state.
func (g Genesis) State() *state.State {
	out := state.New()
	for address, balance := range g.InitialAllocations {
		out.Set(types.Address([]byte(address)), state.Account{Balance: balance})
	}
	return out
}

// Block builds the deterministic height-zero genesis block.
func (g Genesis) Block() (block.Block, error) {
	st := g.State()
	return block.Block{
		Header: block.Header{
			Version:            g.ProtocolVersion,
			ChainID:             g.ChainID,
			Height:              0,
			Timestamp:           g.Timestamp,
			PreviousHash:       types.Hash{},
			TransactionsRoot:   emptyTransactionsRoot(),
			StateRoot:          st.Root(),
			Proposer:            nil,
			ConsensusEvidence: nil,
		},
		Transactions: nil,
	}, nil
}

// Hash returns a deterministic development identity of the genesis configuration.
// This is separate from the block hash and is not the final canonical genesis ID.
func (g Genesis) Hash() types.Hash {
	var b bytes.Buffer
	writeU64(&b, uint64(g.ChainID))
	writeString(&b, g.NetworkProfile)
	writeU16(&b, uint16(g.ProtocolVersion))
	writeI64(&b, g.Timestamp)

	validators := cloneBytes(g.InitialValidators)
	sort.Slice(validators, func(i, j int) bool { return bytes.Compare(validators[i], validators[j]) < 0 })
	writeBytesList(&b, validators)

	accounts := cloneBytes(g.InitialAccounts)
	sort.Slice(accounts, func(i, j int) bool { return bytes.Compare(accounts[i], accounts[j]) < 0 })
	writeBytesList(&b, accounts)

	writeUint64Map(&b, g.InitialAllocations)
	writeUint64Map(&b, g.ProtocolParameters)

	return sha256.Sum256(b.Bytes())
}

func emptyTransactionsRoot() types.Hash {
	return sha256.Sum256(nil)
}

func cloneBytes(in [][]byte) [][]byte {
	out := make([][]byte, len(in))
	for i, value := range in {
		out[i] = append([]byte(nil), value...)
	}
	return out
}

func writeBytesList(b *bytes.Buffer, values [][]byte) {
	writeU32(b, uint32(len(values)))
	for _, value := range values {
		writeBytes(b, value)
	}
}

func writeUint64Map(b *bytes.Buffer, values map[string]uint64) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	writeU32(b, uint32(len(keys)))
	for _, key := range keys {
		writeString(b, key)
		writeU64(b, values[key])
	}
}

func writeString(b *bytes.Buffer, value string) {
	writeBytes(b, []byte(value))
}

func writeBytes(b *bytes.Buffer, value []byte) {
	writeU32(b, uint32(len(value)))
	_, _ = b.Write(value)
}

func writeU16(b *bytes.Buffer, value uint16) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], value)
	_, _ = b.Write(buf[:])
}

func writeU32(b *bytes.Buffer, value uint32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], value)
	_, _ = b.Write(buf[:])
}

func writeU64(b *bytes.Buffer, value uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], value)
	_, _ = b.Write(buf[:])
}

func writeI64(b *bytes.Buffer, value int64) {
	writeU64(b, uint64(value))
}
