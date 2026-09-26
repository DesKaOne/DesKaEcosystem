package consensus

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrFinalizedBlockMismatch = errors.New("finalized block does not match finality certificate")
)

// ValidateFinalizedBlock checks that a block candidate is structurally valid
// for the supplied production context and that its deterministic development
// hash is exactly the payload attested by the finality certificate.
//
// This boundary validates authority-to-block binding only. It does not execute
// or commit canonical state.
func ValidateFinalizedBlock(
	ctx BlockProductionContext,
	candidate block.Block,
	certificate FinalityCertificate,
	validators ValidatorSet,
	votingPower VotingPowerSet,
) (types.Hash, error) {
	payload, err := ValidateProducedBlock(ctx, candidate)
	if err != nil {
		return types.Hash{}, err
	}
	if err := ValidateFinalityCertificate(certificate, ctx.State, validators, votingPower); err != nil {
		return types.Hash{}, err
	}
	if !bytes.Equal(payload[:], certificate.Payload) {
		return types.Hash{}, fmt.Errorf("%w: certificate payload does not match block hash", ErrFinalizedBlockMismatch)
	}
	return payload, nil
}

// FinalizedBlockCommitter is the narrow execution/commit boundary used after
// consensus has produced a valid finality certificate. Consensus owns
// authority validation; the implementation owns deterministic block execution
// and canonical-state commit.
type FinalizedBlockCommitter interface {
	CommitFinalizedBlock(candidate block.Block, certificate FinalityCertificate) error
}
