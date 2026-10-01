package storage

import (
	"errors"
	"sync"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrNilCandidateStore = errors.New("nil candidate store")
	ErrCandidateNotFound = errors.New("candidate not found")
	ErrCandidateKeyMismatch = errors.New("candidate key mismatch")
)

type CandidateKey struct {
	Height types.Height
	Hash   types.Hash
}

type CandidateStore interface {
	SaveCandidate(CandidateKey, block.Block) error
	GetCandidate(CandidateKey) (block.Block, error)
}

type MemoryCandidateStore struct {
	mu    sync.RWMutex
	items map[CandidateKey]block.Block
}

func NewMemoryCandidateStore() *MemoryCandidateStore {
	return &MemoryCandidateStore{items: make(map[CandidateKey]block.Block)}
}

func (s *MemoryCandidateStore) SaveCandidate(key CandidateKey, candidate block.Block) error {
	if s == nil { return ErrNilCandidateStore }
	if err := validateCandidateKey(key, candidate); err != nil { return err }
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil { s.items = make(map[CandidateKey]block.Block) }
	s.items[key] = cloneBlock(candidate)
	return nil
}

func (s *MemoryCandidateStore) GetCandidate(key CandidateKey) (block.Block, error) {
	if s == nil { return block.Block{}, ErrNilCandidateStore }
	s.mu.RLock()
	defer s.mu.RUnlock()
	candidate, ok := s.items[key]
	if !ok { return block.Block{}, ErrCandidateNotFound }
	return cloneBlock(candidate), nil
}

func validateCandidateKey(key CandidateKey, candidate block.Block) error {
	if candidate.Header.Height != key.Height { return ErrCandidateKeyMismatch }
	hash, err := block.Hash(candidate)
	if err != nil { return err }
	if hash != key.Hash { return ErrCandidateKeyMismatch }
	return nil
}

func cloneBlock(b block.Block) block.Block {
	out := b
	out.Header.Proposer = append([]byte(nil), b.Header.Proposer...)
	out.Header.ConsensusEvidence = append([]byte(nil), b.Header.ConsensusEvidence...)
	out.Transactions = append([]any(nil), b.Transactions...)
	return out
}

var _ CandidateStore = (*MemoryCandidateStore)(nil)
