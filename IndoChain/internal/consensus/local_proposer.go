package consensus

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/mempool"
)

var (
	ErrNilLocalBlockProducer = errors.New("nil local block producer")
	ErrInvalidLocalProducer  = errors.New("invalid local block producer")
)

// LocalBlockProducer connects the deterministic mempool snapshot to block
// construction. It owns no clock, scheduler, consensus state, or canonical
// commit path; the caller supplies the consensus context and timestamp.
type LocalBlockProducer struct {
	pool           *mempool.Pool
	canonicalState *state.State
	rules          block.ExecutionRules
	timestamp      func() int64
}

// NewLocalBlockProducer creates the development local proposer boundary.
// timestamp is injected so tests and callers control proposal time explicitly.
func NewLocalBlockProducer(
	pool *mempool.Pool,
	canonicalState *state.State,
	rules block.ExecutionRules,
	timestamp func() int64,
) (*LocalBlockProducer, error) {
	if pool == nil || canonicalState == nil || timestamp == nil {
		return nil, ErrInvalidLocalProducer
	}
	return &LocalBlockProducer{
		pool: pool, canonicalState: canonicalState, rules: rules, timestamp: timestamp,
	}, nil
}

func (p *LocalBlockProducer) CanonicalState() *state.State {
	if p == nil {
		return nil
	}
	return p.canonicalState
}

func (p *LocalBlockProducer) ExecutionRules() block.ExecutionRules {
	if p == nil {
		return block.ExecutionRules{}
	}
	return p.rules
}

// ProduceBlock builds one candidate from the current sorted mempool snapshot,
// executes it against a state snapshot, and never mutates canonical state.
// Invalid transactions are rejected as a batch rather than silently filtered.
func (p *LocalBlockProducer) ProduceBlock(ctx BlockProductionContext) (block.Block, error) {
	if p == nil || p.pool == nil || p.canonicalState == nil || p.timestamp == nil {
		return block.Block{}, ErrNilLocalBlockProducer
	}
	if err := ctx.State.Validate(); err != nil {
		return block.Block{}, err
	}
	if len(ctx.Proposer) == 0 {
		return block.Block{}, ErrInvalidBlockProductionContext
	}
	if p.rules.ChainID != ctx.State.ChainID ||
		p.rules.ProtocolVersion != ctx.State.ProtocolVersion {
		return block.Block{}, ErrBlockProductionContextMismatch
	}

	return BuildCandidateFromTransactions(
		ctx,
		p.pool.SnapshotSorted(),
		p.canonicalState,
		p.rules,
		p.timestamp(),
	)
}

// BuildCandidateFromTransactions is the lower-level deterministic builder for
// callers that already selected transactions. It never mutates canonicalState.
func BuildCandidateFromTransactions(
	ctx BlockProductionContext,
	txs []transaction.Transaction,
	canonicalState *state.State,
	rules block.ExecutionRules,
	timestamp int64,
) (block.Block, error) {
	if canonicalState == nil {
		return block.Block{}, ErrInvalidLocalProducer
	}
	if err := ctx.State.Validate(); err != nil {
		return block.Block{}, err
	}
	if len(ctx.Proposer) == 0 {
		return block.Block{}, ErrInvalidBlockProductionContext
	}
	if rules.ChainID != ctx.State.ChainID ||
		rules.ProtocolVersion != ctx.State.ProtocolVersion {
		return block.Block{}, ErrBlockProductionContextMismatch
	}

	raw := make([]any, len(txs))
	for i := range txs {
		raw[i] = txs[i]
	}
	txRoot, err := block.TransactionsRoot(raw)
	if err != nil {
		return block.Block{}, err
	}

	candidate := block.Block{
		Header: block.Header{
			Version:          ctx.State.ProtocolVersion,
			ChainID:           ctx.State.ChainID,
			Height:            ctx.State.Height + 1,
			Timestamp:         timestamp,
			PreviousHash:      ctx.PreviousHash,
			TransactionsRoot:  txRoot,
			Proposer:          append([]byte(nil), ctx.Proposer...),
		},
		Transactions: raw,
	}

	working := canonicalState.Snapshot()
	if err := block.ExecuteBlock(
		working,
		candidate,
		candidate.Header.Height,
		ctx.PreviousHash,
		rules,
	); err != nil {
		return block.Block{}, err
	}
	candidate.Header.StateRoot = working.Root()

	if _, err := ValidateProducedBlock(ctx, candidate); err != nil {
		return block.Block{}, err
	}
	return candidate, nil
}

var _ BlockProducer = (*LocalBlockProducer)(nil)
