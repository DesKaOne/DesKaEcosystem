package node

import (
    "errors"
    "fmt"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/p2p"
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
    node       *Node
    engine     *consensus.ConsensusEngine
    transport  p2p.Transport
    rules      consensus.ValidationRules
    peers      []p2p.PeerID
    ctx        consensus.BlockProductionContext
    candidate  *block.Block
    validators consensus.ValidatorSet
    power      consensus.VotingPowerSet
    validatorResolver ValidatorAuthorityResolver
    senderResolver TransactionAuthorityResolver
    committed bool
}

func NewConsensusSession(
    n *Node,
    engine *consensus.ConsensusEngine,
    transport p2p.Transport,
    rules consensus.ValidationRules,
    peers []p2p.PeerID,
    ctx consensus.BlockProductionContext,
    validators consensus.ValidatorSet,
    power consensus.VotingPowerSet,
    validatorResolver ValidatorAuthorityResolver,
    senderResolver TransactionAuthorityResolver,
) (*ConsensusSession, error) {
    if n == nil || engine == nil {
        return nil, ErrNilConsensusSession
    }
    if transport == nil {
        return nil, p2p.ErrNilTransport
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
        return nil, ErrConsensusContextMismatch
    }
    return &ConsensusSession{
        node: n,
        engine: engine,
        transport: transport,
        rules: rules,
        peers: append([]p2p.PeerID(nil), peers...),
        ctx: ctx,
        validators: validators,
        power: power,
        validatorResolver: validatorResolver,
        senderResolver: senderResolver,
    }, nil
}

// StartProposal validates the candidate against the node's canonical state,
// signs the proposal through the ConsensusEngine, consumes it locally, and
// broadcasts the resulting proposal + locally generated vote events.
func (s *ConsensusSession) StartProposal(candidate block.Block) error {
    if s == nil || s.node == nil || s.engine == nil {
        return ErrNilConsensusSession
    }
    if s.committed {
        return errors.New("consensus session already committed")
    }
    rules, err := s.node.Config.BlockRules(nil)
    if err != nil {
        return err
    }
    rules.Transaction.PublicKeyResolver = s.senderResolver
    proposal, err := s.engine.BuildProposalMessage(candidate)
    if err != nil {
        return err
    }
    if err := s.engine.Runtime().ValidateAuthenticatedBlockProposal(
        proposal,
        candidate,
        s.ctx,
        s.node.State,
        rules,
        s.validatorResolver,
    ); err != nil {
        return err
    }
    // The candidate is retained only as the exact block bound to the current
    // proposal hash; it is never treated as canonical before finality commit.
    s.candidate = cloneCandidate(candidate)
    generated, err := s.engine.ProcessMessage(proposal)
    if err != nil {
        return err
    }
    if err := s.broadcast(proposal); err != nil {
        return err
    }
    return s.broadcastGenerated(generated)
}

// HandlePeerMessage processes one authenticated consensus event received from
// a peer. A proposal must carry its separately exchanged candidate so the
// node can execute/validate the exact block before the consensus phase moves.
func (s *ConsensusSession) HandlePeerMessage(from p2p.PeerID, msg consensus.Message, candidate *block.Block) error {
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
        rules, err := s.node.Config.BlockRules(nil)
        if err != nil {
            return err
        }
        rules.Transaction.PublicKeyResolver = s.senderResolver
        if err := s.engine.Runtime().ValidateAuthenticatedBlockProposal(
            msg, *candidate, s.ctx, s.node.State, rules, s.validatorResolver,
        ); err != nil {
            return err
        }
        s.candidate = cloneCandidate(*candidate)
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

// ReceiveAndProcess receives exactly one consensus transport event and routes
// it through HandlePeerMessage. Candidate exchange remains an explicit P2P
// boundary: callers provide the already hash-verified candidate for proposals.
func (s *ConsensusSession) ReceiveAndProcess(candidate *block.Block) (p2p.PeerID, error) {
    if s == nil || s.transport == nil {
        return "", ErrNilConsensusSession
    }
    from, msg, err := p2p.ReceiveConsensus(s.transport, s.rules)
    if err != nil {
        return from, err
    }
    return from, s.HandlePeerMessage(from, msg, candidate)
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
        if err := p2p.SendConsensus(s.transport, peer, msg, s.rules); err != nil {
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

func cloneCandidate(candidate block.Block) *block.Block {
    out := candidate
    out.Header.Proposer = append([]byte(nil), candidate.Header.Proposer...)
    out.Header.ConsensusEvidence = append([]byte(nil), candidate.Header.ConsensusEvidence...)
    out.Transactions = append([]any(nil), candidate.Transactions...)
    return &out
}

