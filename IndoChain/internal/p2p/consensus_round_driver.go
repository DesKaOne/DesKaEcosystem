package p2p

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
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
