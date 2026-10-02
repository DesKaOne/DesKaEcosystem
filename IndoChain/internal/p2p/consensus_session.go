package p2p

import (
    "errors"
    "fmt"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/node"
)

var (
    ErrNilConsensusSession = errors.New("nil node consensus session")
    ErrConsensusSessionCandidateRequired = errors.New("consensus proposal candidate required")
    ErrConsensusSessionPeerRequired = errors.New("consensus session peer required")
)

// ConsensusSession is the node-owned operational bridge between the
// authenticated ConsensusEngine, P2P consensus transport, and canonical
// finality commit boundary. It does not own consensus clocks or canonical
// storage semantics.
type ConsensusSession struct {
    node       *node.Node
    engine     *consensus.ConsensusEngine
    transport  Transport
    rules      consensus.ValidationRules
    peers      []PeerID
    ctx        consensus.BlockProductionContext
    candidate  *block.Block
    validators consensus.ValidatorSet
    power      consensus.VotingPowerSet
    validatorResolver node.ValidatorAuthorityResolver
    senderResolver node.TransactionAuthorityResolver
    committed bool
}

func NewConsensusSession(
    n *node.Node,
    engine *consensus.ConsensusEngine,
    transport Transport,
    rules consensus.ValidationRules,
    peers []PeerID,
    ctx consensus.BlockProductionContext,
    validators consensus.ValidatorSet,
    power consensus.VotingPowerSet,
    validatorResolver node.ValidatorAuthorityResolver,
    senderResolver node.TransactionAuthorityResolver,
) (*ConsensusSession, error) {
    if n == nil || engine == nil {
        return nil, ErrNilConsensusSession
    }
    if transport == nil {
        return nil, ErrNilTransport
    }
    if len(peers) == 0 {
        return nil, ErrConsensusSessionPeerRequired
    }
    if validatorResolver == nil || senderResolver == nil {
        return nil, errors.New("missing consensus session authority resolver")
    }
    if err := ctx.State.Validate(); err != nil {
        return nil, err
    }
    if ctx.State.Height != n.Head.Header.Height || ctx.PreviousHash != n.HeadHash {
        return nil, node.ErrConsensusContextMismatch
    }
    return &ConsensusSession{
        node: n, engine: engine, transport: transport, rules: rules,
        peers: append([]PeerID(nil), peers...), ctx: ctx,
        validators: validators, power: power,
        validatorResolver: validatorResolver, senderResolver: senderResolver,
    }, nil
}

func (s *ConsensusSession) StartProposal(candidate block.Block) error {
    if s == nil || s.node == nil || s.engine == nil {
        return ErrNilConsensusSession
    }
    if s.committed {
        return errors.New("consensus session already committed")
    }
    proposal, err := s.engine.BuildProposalMessage(candidate)
    if err != nil {
        return err
    }
    if err := s.validateProposalCandidate(proposal, candidate); err != nil {
        return err
    }
    s.candidate = cloneConsensusCandidate(candidate)
    generated, err := s.engine.ProcessMessage(proposal)
    if err != nil {
        return err
    }
    if err := s.broadcast(proposal); err != nil {
        return err
    }
    return s.broadcastGenerated(generated)
}

func (s *ConsensusSession) HandlePeerMessage(from PeerID, msg consensus.Message, candidate *block.Block) error {
    if s == nil || s.engine == nil || s.node == nil {
        return ErrNilConsensusSession
    }
    if from == "" {
        return ErrConsensusSessionPeerRequired
    }
    if msg.Type == consensus.MessageTypeProposal {
        if candidate == nil {
            return ErrConsensusSessionCandidateRequired
        }
        if err := s.validateProposalCandidate(msg, *candidate); err != nil {
            return err
        }
        s.candidate = cloneConsensusCandidate(*candidate)
    }
    generated, err := s.engine.ProcessMessage(msg)
    if err != nil {
        return err
    }
    if err := s.broadcastGenerated(generated); err != nil {
        return err
    }
    return s.commitIfFinalized()
}

func (s *ConsensusSession) ReceiveAndProcess(candidate *block.Block) (PeerID, error) {
    if s == nil || s.transport == nil {
        return "", ErrNilConsensusSession
    }
    from, msg, err := ReceiveConsensus(s.transport, s.rules)
    if err != nil {
        return from, err
    }
    return from, s.HandlePeerMessage(from, msg, candidate)
}

func (s *ConsensusSession) validateProposalCandidate(msg consensus.Message, candidate block.Block) error {
    rules, err := s.node.Config.BlockRules(nil)
    if err != nil {
        return err
    }
    rules.Transaction.PublicKeyResolver = s.senderResolver
    if err := consensus.ValidateBlockCandidateContext(msg, s.ctx, candidate); err != nil {
        return err
    }
    payload, err := consensus.ValidateBlockCandidateForConsensus(s.ctx, candidate, s.node.State, rules)
    if err != nil {
        return err
    }
    if string(payload[:]) != string(msg.Payload) {
        return consensus.ErrCandidatePayloadMismatch
    }
    publicKey, err := s.validatorResolver.PublicKeyForValidator(msg.Sender)
    if err != nil {
        return err
    }
    return consensus.VerifyMessageSignature(msg, publicKey)
}

func (s *ConsensusSession) commitIfFinalized() error {
    if s.committed {
        return nil
    }
    if s.candidate == nil {
        return errors.New("finalized consensus candidate missing")
    }
    certificate, err := s.engine.Runtime().FinalizedCertificate()
    if err != nil {
        return nil
    }
    if err := s.node.CommitFinalityEvidenceAndPublishConsensus(
        s.ctx, *s.candidate, certificate, s.validators, s.power,
        s.validatorResolver, s.senderResolver, s.engine.Runtime(),
    ); err != nil {
        return fmt.Errorf("canonical consensus commit: %w", err)
    }
    s.committed = true
    s.candidate = nil
    return nil
}

func (s *ConsensusSession) broadcast(msg consensus.Message) error {
    for _, peer := range s.peers {
        if err := SendConsensus(s.transport, peer, msg, s.rules); err != nil {
            return err
        }
    }
    return nil
}

func (s *ConsensusSession) broadcastGenerated(messages []consensus.Message) error {
    for _, msg := range messages {
        if err := s.broadcast(msg); err != nil {
            return err
        }
    }
    return nil
}

func cloneConsensusCandidate(candidate block.Block) *block.Block {
    out := candidate
    out.Header.Proposer = append([]byte(nil), candidate.Header.Proposer...)
    out.Header.ConsensusEvidence = append([]byte(nil), candidate.Header.ConsensusEvidence...)
    out.Transactions = append([]any(nil), candidate.Transactions...)
    return &out
}
