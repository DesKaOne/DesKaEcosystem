package consensus

import (
    "testing"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestValidateBlockCandidateForConsensusDoesNotMutateCanonicalState(t *testing.T) {
    rs, err := NewRoundState(1, 1, 0, 7)
    if err != nil { t.Fatal(err) }
    canonical := state.New()
    before := canonical.Root()
    proposer := []byte("validator-a")
    ctx := BlockProductionContext{State:rs, PreviousHash:types.Hash{9}, Proposer:proposer}
    candidate := block.Block{Header:block.Header{
        Version:1, ChainID:1, Height:8, PreviousHash:types.Hash{9},
        TransactionsRoot:types.Hash{}, StateRoot:canonical.Root(), Proposer:proposer,
    }}
    got, err := ValidateBlockCandidateForConsensus(ctx, candidate, canonical, block.ExecutionRules{ChainID:1, ProtocolVersion:1})
    if err != nil { t.Fatal(err) }
    expected, err := block.Hash(candidate)
    if err != nil { t.Fatal(err) }
    if got != expected { t.Fatalf("payload hash mismatch") }
    if canonical.Root() != before { t.Fatalf("canonical state mutated") }
}

func TestValidateBlockCandidateForConsensusRejectsStateRootMismatchWithoutMutation(t *testing.T) {
    rs, err := NewRoundState(1, 1, 0, 7)
    if err != nil { t.Fatal(err) }
    canonical := state.New()
    before := canonical.Root()
    candidate := block.Block{Header:block.Header{
        Version:1, ChainID:1, Height:8, PreviousHash:types.Hash{9},
        TransactionsRoot:types.Hash{}, StateRoot:types.Hash{7}, Proposer:[]byte("validator-a"),
    }}
    _, err = ValidateBlockCandidateForConsensus(
        BlockProductionContext{State:rs, PreviousHash:types.Hash{9}, Proposer:[]byte("validator-a")},
        candidate, canonical, block.ExecutionRules{ChainID:1, ProtocolVersion:1},
    )
    if err != block.ErrStateRootMismatch { t.Fatalf("error=%v", err) }
    if canonical.Root() != before { t.Fatalf("canonical state mutated after rejection") }
}

func TestValidateBlockCandidateContextRejectsProposalCandidateMismatch(t *testing.T) {
    rs, err := NewRoundState(1, 1, 0, 7)
    if err != nil { t.Fatal(err) }
    msg := Message{ProtocolVersion:1, ChainID:1, Epoch:0, Height:7, Round:0, Sender:[]byte("validator-a"), Type:MessageTypeProposal, Payload:make([]byte, 32)}
    candidate := block.Block{Header:block.Header{
        Version:1, ChainID:1, Height:8, PreviousHash:types.Hash{8}, Proposer:[]byte("validator-a"),
    }}
    err = ValidateBlockCandidateContext(msg, BlockProductionContext{State:rs, PreviousHash:types.Hash{9}, Proposer:[]byte("validator-a")}, candidate)
    if err != ErrBlockProductionContextMismatch { t.Fatalf("error=%v", err) }
}
