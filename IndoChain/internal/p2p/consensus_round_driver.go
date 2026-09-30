package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

var (
	ErrNilConsensusRoundDriver = errors.New("nil p2p consensus round driver")
)

// ConsensusRoundDriver binds the authenticated consensus RoundDriver to the
// existing node-to-node transport. It deliberately does not own clocks,
// peer discovery, retransmission, or canonical block storage.
type ConsensusRoundDriver struct {
	transport Transport
	driver   *consensus.RoundDriver
	rules    consensus.ValidationRules
	candidate *CandidateExchange
}

func NewConsensusRoundDriver(
	transport Transport,
	runtime *consensus.ValidatorRuntime,
	authority consensus.TimeoutAuthorityResolver,
	rules consensus.ValidationRules,
) (*ConsensusRoundDriver, error) {
	if transport == nil {
		return nil, ErrNilTransport
	}
	if runtime == nil {
		return nil, ErrNilConsensusRoundDriver
	}
	driver, err := consensus.NewRoundDriver(runtime, authority)
	if err != nil {
		return nil, err
	}
	return &ConsensusRoundDriver{transport: transport, driver: driver, rules: rules}, nil
}

func (d *ConsensusRoundDriver) Runtime() *consensus.ValidatorRuntime {
	if d == nil || d.driver == nil {
		return nil
	}
	return d.driver.Runtime()
}

// Publish sends an already authenticated consensus message.
func (d *ConsensusRoundDriver) Publish(peer PeerID, msg consensus.Message) error {
	if d == nil || d.driver == nil {
		return ErrNilConsensusRoundDriver
	}
	return SendConsensus(d.transport, peer, msg, d.rules)
}

// PublishBlockProposal builds and signs the deterministic proposal evidence,
// then sends it through the consensus transport. The full block candidate is
// intentionally not serialized here; its canonical wire encoding remains a
// separate protocol boundary.
func (d *ConsensusRoundDriver) PublishBlockProposal(
	peer PeerID,
	ctx consensus.BlockProductionContext,
	candidate block.Block,
	signer crypto.Signer,
) (consensus.Message, consensus.BlockProposal, error) {
	if d == nil || d.driver == nil {
		return consensus.Message{}, consensus.BlockProposal{}, ErrNilConsensusRoundDriver
	}
	msg, proposal, err := consensus.BuildSignedProposalMessage(ctx, candidate, signer)
	if err != nil {
		return consensus.Message{}, consensus.BlockProposal{}, err
	}
	if err := d.Publish(peer, msg); err != nil {
		return consensus.Message{}, consensus.BlockProposal{}, err
	}
	return msg, proposal, nil
}

// ReceiveAndHandle processes exactly one transport message through the
// authenticated RoundDriver. Invalid signatures/context never reach runtime
// aggregation or phase mutation.
func (d *ConsensusRoundDriver) ReceiveAndHandle() (PeerID, error) {
	if d == nil || d.driver == nil {
		return "", ErrNilConsensusRoundDriver
	}
	from, msg, err := ReceiveConsensus(d.transport, d.rules)
	if err != nil {
		return from, err
	}
	return from, d.driver.HandleMessage(msg)
}

// NewConsensusRoundDriverWithCandidateExchange adds the existing block/sync
// candidate exchange without changing the basic consensus-driver constructor.
func NewConsensusRoundDriverWithCandidateExchange(
	transport Transport,
	runtime *consensus.ValidatorRuntime,
	authority consensus.TimeoutAuthorityResolver,
	rules consensus.ValidationRules,
	maxPayload uint32,
	maxRequest uint64,
) (*ConsensusRoundDriver, error) {
	d, err := NewConsensusRoundDriver(transport, runtime, authority, rules)
	if err != nil { return nil, err }
	exchange, err := NewCandidateExchange(transport, maxPayload, maxRequest)
	if err != nil { return nil, err }
	d.candidate = exchange
	return d, nil
}

// PublishBlockProposalAndCandidate publishes authenticated proposal evidence
// and the complete development candidate through the existing block message.
func (d *ConsensusRoundDriver) PublishBlockProposalAndCandidate(
	peer PeerID,
	ctx consensus.BlockProductionContext,
	candidate block.Block,
	signer crypto.Signer,
) (consensus.Message, consensus.BlockProposal, error) {
	if d == nil || d.driver == nil || d.candidate == nil {
		return consensus.Message{}, consensus.BlockProposal{}, ErrNilConsensusRoundDriver
	}
	msg, proposal, err := d.PublishBlockProposal(peer, ctx, candidate, signer)
	if err != nil { return consensus.Message{}, consensus.BlockProposal{}, err }
	if err := d.candidate.PublishCandidate(peer, candidate); err != nil {
		return consensus.Message{}, consensus.BlockProposal{}, err
	}
	return msg, proposal, nil
}

// FetchProposalCandidate fetches one candidate at the proposal height and
// verifies its deterministic block hash against the authenticated proposal.
func (d *ConsensusRoundDriver) FetchProposalCandidate(peer PeerID, proposal consensus.Message) (block.Block, error) {
	if d == nil || d.candidate == nil { return block.Block{}, ErrNilConsensusRoundDriver }
	return d.candidate.FetchCandidate(peer, proposal)
}

// AcceptFetchedBlockProposal validates the fetched candidate against the
// authenticated proposal, executes it on a snapshot, and only then advances
// the consensus runtime into Prevote.
func (d *ConsensusRoundDriver) AcceptFetchedBlockProposal(
	proposal consensus.Message,
	candidate block.Block,
	ctx consensus.BlockProductionContext,
	canonicalState *state.State,
	executionRules block.ExecutionRules,
) error {
	if d == nil || d.driver == nil {
		return ErrNilConsensusRoundDriver
	}
	return d.driver.Runtime().ValidateAuthenticatedBlockProposal(
		proposal,
		candidate,
		ctx,
		canonicalState,
		executionRules,
		d.driver.Authority(),
	)
}

// PublishLocalProposalAndPrevote builds a candidate from the deterministic
// mempool snapshot, publishes the authenticated proposal and candidate,
// validates the same candidate locally, then signs and emits the local
// prevote. No scheduler, clock, retransmission, or canonical commit is owned
// by this method; proposal time comes from the LocalBlockProducer.
func (d *ConsensusRoundDriver) PublishLocalProposalAndPrevote(
	peer PeerID,
	ctx consensus.BlockProductionContext,
	producer *consensus.LocalBlockProducer,
	signer crypto.Signer,
) (consensus.Message, consensus.Message, block.Block, error) {
	if d == nil || d.driver == nil || d.candidate == nil {
		return consensus.Message{}, consensus.Message{}, block.Block{}, ErrNilConsensusRoundDriver
	}
	if producer == nil || signer == nil {
		return consensus.Message{}, consensus.Message{}, block.Block{}, consensus.ErrInvalidLocalProducer
	}

	candidate, err := producer.ProduceBlock(ctx)
	if err != nil {
		return consensus.Message{}, consensus.Message{}, block.Block{}, err
	}
	proposalMsg, _, err := d.PublishBlockProposalAndCandidate(peer, ctx, candidate, signer)
	if err != nil {
		return consensus.Message{}, consensus.Message{}, block.Block{}, err
	}

	if err := d.AcceptFetchedBlockProposal(
		proposalMsg,
		candidate,
		ctx,
		producer.CanonicalState(),
		producer.ExecutionRules(),
	); err != nil {
		return consensus.Message{}, consensus.Message{}, block.Block{}, err
	}

	prevote := consensus.Message{
		ProtocolVersion: ctx.State.ProtocolVersion,
		ChainID:         ctx.State.ChainID,
		Epoch:           ctx.State.Epoch,
		Height:          ctx.State.Height,
		Round:           ctx.State.Round,
		Sender:          append([]byte(nil), ctx.Proposer...),
		Type:            consensus.MessageTypePrevote,
		Payload:          proposalMsg.Payload,
	}
	prevote, err = prevote.Sign(signer)
	if err != nil {
		return consensus.Message{}, consensus.Message{}, block.Block{}, err
	}

	if err := d.driver.Runtime().AddAuthenticatedVote(prevote, d.driver.Authority()); err != nil {
		return consensus.Message{}, consensus.Message{}, block.Block{}, err
	}
	if err := d.Publish(peer, prevote); err != nil {
		return consensus.Message{}, consensus.Message{}, block.Block{}, err
	}
	return proposalMsg, prevote, candidate, nil
}

// PublishLocalPrecommitAfterPrevote submits the local authenticated prevote
// to the runtime. A precommit is emitted only when that prevote, together with
// already authenticated peer prevotes, causes the runtime to enter Precommit.
// The method does not finalize or commit canonical state.
func (d *ConsensusRoundDriver) PublishLocalPrecommitAfterPrevote(
	peer PeerID,
	prevote consensus.Message,
	signer crypto.Signer,
) (consensus.Message, error) {
	if d == nil || d.driver == nil {
		return consensus.Message{}, ErrNilConsensusRoundDriver
	}
	if signer == nil {
		return consensus.Message{}, consensus.ErrNilProposalSigner
	}
	if prevote.Type != consensus.MessageTypePrevote {
		return consensus.Message{}, consensus.ErrInvalidRuntimeVoteType
	}

	if err := d.driver.Runtime().AddAuthenticatedVote(prevote, d.driver.Authority()); err != nil {
		return consensus.Message{}, err
	}
	if d.driver.Runtime().State().Phase != consensus.PhasePrecommit {
		return consensus.Message{}, nil
	}

	precommit, err := consensus.BuildSignedVoteMessage(
		d.driver.Runtime().State(),
		prevote.Sender,
		consensus.MessageTypePrecommit,
		d.driver.Runtime().Proposal(),
		signer,
	)
	if err != nil {
		return consensus.Message{}, err
	}
	if err := d.driver.Runtime().AddAuthenticatedVote(precommit, d.driver.Authority()); err != nil {
		return consensus.Message{}, err
	}
	if err := d.Publish(peer, precommit); err != nil {
		return consensus.Message{}, err
	}
	return precommit, nil
}

// ServeCandidateRequest handles one existing block-request message.
func (d *ConsensusRoundDriver) ServeCandidateRequest(reader SyncReader) (PeerID, error) {
	if d == nil || d.candidate == nil { return "", ErrNilConsensusRoundDriver }
	return d.candidate.ServeOneRequest(reader)
}

func (d *ConsensusRoundDriver) RoundDriver() *consensus.RoundDriver {
	if d == nil {
		return nil
	}
	return d.driver
}
