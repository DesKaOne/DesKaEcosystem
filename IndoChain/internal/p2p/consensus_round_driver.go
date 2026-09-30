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

func (d *ConsensusRoundDriver) RoundDriver() *consensus.RoundDriver {
	if d == nil {
		return nil
	}
	return d.driver
}
