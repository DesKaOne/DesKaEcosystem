package p2p

import (
    "bytes"
    "errors"
    "fmt"
    "sync"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/node"
)

var (
    ErrNilConsensusSession = errors.New("nil node consensus session")
    ErrConsensusSessionCandidateRequired = errors.New("consensus proposal candidate required")
    ErrConsensusSessionPeerRequired = errors.New("consensus session peer required")
    ErrConsensusSessionProposalProducerRequired = errors.New("consensus proposal producer required")
)

// ConsensusSession is the node-owned operational bridge between the
// authenticated ConsensusEngine, P2P consensus transport, timeout scheduler,
// and canonical finality commit boundary.
type ConsensusSession struct {
    node       *node.Node
    engine     *consensus.ConsensusEngine
    transport  Transport
    rules      consensus.ValidationRules
    peers      []PeerID
    ctx        consensus.BlockProductionContext
    candidate  *block.Block
    pending    map[string]block.Block
    validators consensus.ValidatorSet
    power      consensus.VotingPowerSet
    validatorResolver node.ValidatorAuthorityResolver
    senderResolver node.TransactionAuthorityResolver
    committed bool
    mu sync.Mutex
    scheduler *consensus.ConsensusTimeoutScheduler
    producer consensus.BlockProducer
    started bool
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
    return NewConsensusSessionWithScheduler(
        n, engine, transport, rules, peers, ctx, validators, power,
        validatorResolver, senderResolver, nil,
    )
}

func NewConsensusSessionWithScheduler(
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
    timerFactory consensus.ConsensusTimeoutTimerFactory,
) (*ConsensusSession, error) {
    return NewConsensusSessionWithSchedulerAndProducer(
        n, engine, transport, rules, peers, ctx, validators, power,
        validatorResolver, senderResolver, timerFactory, nil,
    )
}

func NewConsensusSessionWithSchedulerAndProducer(
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
    timerFactory consensus.ConsensusTimeoutTimerFactory,
    producer consensus.BlockProducer,
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
    scheduler, err := consensus.NewConsensusTimeoutScheduler(timerFactory)
    if err != nil {
        return nil, err
    }
    return &ConsensusSession{
        node: n, engine: engine, transport: transport, rules: rules,
        peers: append([]PeerID(nil), peers...), ctx: ctx,
        validators: validators, power: power,
        validatorResolver: validatorResolver, senderResolver: senderResolver,
        pending: make(map[string]block.Block),
        scheduler: scheduler,
        producer: producer,
    }, nil
}

// Start activates the external timeout lifecycle for the session.
func (s *ConsensusSession) Start() error {
    if s == nil || s.engine == nil || s.scheduler == nil {
        return ErrNilConsensusSession
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.committed {
        return errors.New("consensus session already committed")
    }
    if s.started {
        return nil
    }
    s.started = true
    return s.armTimeoutLocked()
}

// Stop fences the scheduler and invalidates the engine timeout before a
// session shutdown/restart. It does not mutate canonical state.
func (s *ConsensusSession) Stop() {
    if s == nil {
        return
    }
    s.mu.Lock()
    s.started = false
    scheduler := s.scheduler
    engine := s.engine
    s.mu.Unlock()
    if scheduler != nil {
        _ = scheduler.CancelEngineTimeout(engine)
    }
}

func (s *ConsensusSession) StartProposal(candidate block.Block) error {
    if s == nil || s.node == nil || s.engine == nil {
        return ErrNilConsensusSession
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.committed {
        return errors.New("consensus session already committed")
    }
    return s.startProposalLocked(candidate)
}

func (s *ConsensusSession) startProposalLocked(candidate block.Block) error {
    s.ctx.State = s.engine.Runtime().State()
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
    if err := s.broadcastCandidate(candidate); err != nil {
        return err
    }
    if err := s.broadcast(proposal); err != nil {
        return err
    }
    if err := s.broadcastGenerated(generated); err != nil {
        return err
    }
    s.started = true
    return s.armTimeoutLocked()
}

func (s *ConsensusSession) handoffRoundProposalLocked() error {
    expected, err := s.engine.ExpectedProposer()
    if err != nil {
        return err
    }
    if !bytes.Equal(expected, s.engine.ValidatorID()) {
        return nil
    }

    var candidate block.Block
    if s.candidate != nil && bytes.Equal(s.candidate.Header.Proposer, expected) {
        candidate = *cloneConsensusCandidate(*s.candidate)
    } else {
        if s.producer == nil {
            return ErrConsensusSessionProposalProducerRequired
        }
        state := s.engine.Runtime().State()
        candidate, err = s.producer.ProduceBlock(consensus.BlockProductionContext{
            State:        state,
            PreviousHash: s.ctx.PreviousHash,
            Proposer:     append([]byte(nil), expected...),
        })
        if err != nil {
            return err
        }
    }
    return s.startProposalLocked(candidate)
}

func (s *ConsensusSession) handleScheduledTimeout(msg consensus.Message, err error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if err != nil || !s.started || s.committed {
        return
    }
    if err := s.broadcast(msg); err != nil {
        return
    }
    if _, err := s.engine.TryAdvanceRound(); err == nil {
        _ = s.handoffRoundProposalLocked()
    }
}

func (s *ConsensusSession) armTimeoutLocked() error {
    if s.scheduler == nil || s.engine == nil {
        return ErrNilConsensusSession
    }
    if !s.started {
        return nil
    }
    return s.scheduler.ArmEngineTimeout(s.engine, s.handleScheduledTimeout)
}

func (s *ConsensusSession) HandlePeerMessage(from PeerID, msg consensus.Message, candidate *block.Block) error {
    if s == nil || s.engine == nil || s.node == nil {
        return ErrNilConsensusSession
    }
    s.mu.Lock()
    defer s.mu.Unlock()
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
    if err := s.commitIfFinalized(); err != nil {
        return err
    }
    if s.started {
        if _, err := s.engine.TryAdvanceRound(); err == nil {
            return s.handoffRoundProposalLocked()
        }
        return s.armTimeoutLocked()
    }
    return nil
}

func (s *ConsensusSession) PumpOnce() (PeerID, error) {
    if s == nil || s.transport == nil {
        return "", ErrNilConsensusSession
    }
    from, msg, err := s.transport.Receive()
    if err != nil {
        return from, err
    }
    switch msg.Type {
    case MessageTypeBlock:
        candidate, err := DecodeBlockDevelopment(msg.Payload, s.rules.MaxPayloadSize)
        if err != nil {
            return from, err
        }
        hash, err := block.Hash(candidate)
        if err != nil {
            return from, err
        }
        s.mu.Lock()
        s.pending[string(hash[:])] = candidate
        s.mu.Unlock()
        return from, nil
    case MessageTypeConsensus:
        decoded, err := consensus.DecodeMessage(msg.Payload, s.rules)
        if err != nil {
            return from, err
        }
        var candidate *block.Block
        if decoded.Type == consensus.MessageTypeProposal {
            s.mu.Lock()
            pending, ok := s.pending[string(decoded.Payload)]
            s.mu.Unlock()
            if !ok {
                return from, ErrConsensusSessionCandidateRequired
            }
            candidate = &pending
        }
        return from, s.HandlePeerMessage(from, decoded, candidate)
    default:
        return from, ErrUnknownMessage
    }
}

func (s *ConsensusSession) ReceiveAndProcess(candidate *block.Block) (PeerID, error) {
    if s == nil || s.transport == nil {
        return "", ErrNilConsensusSession
    }
    for {
        from, msg, err := s.transport.Receive()
        if err != nil {
            return from, err
        }
        if msg.Type == MessageTypeBlock {
            continue
        }
        if msg.Type != MessageTypeConsensus {
            return from, ErrUnknownMessage
        }
        decoded, err := consensus.DecodeMessage(msg.Payload, s.rules)
        if err != nil {
            return from, err
        }
        return from, s.HandlePeerMessage(from, decoded, candidate)
    }
}

func (s *ConsensusSession) validateProposalCandidate(msg consensus.Message, candidate block.Block) error {
    rules, err := s.node.Config.BlockRules(nil)
    if err != nil {
        return err
    }
    rules.Transaction.PublicKeyResolver = s.senderResolver
    ctx := s.ctx
    ctx.State = s.engine.Runtime().State()
    if err := consensus.ValidateBlockCandidateContext(msg, ctx, candidate); err != nil {
        return err
    }
    payload, err := consensus.ValidateBlockCandidateForConsensus(ctx, candidate, s.node.State, rules)
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
    certificate, err := s.engine.Runtime().FinalizedCertificate()
    if err != nil {
        return nil
    }
    if s.candidate == nil {
        return errors.New("finalized consensus candidate missing")
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

func (s *ConsensusSession) broadcastCandidate(candidate block.Block) error {
    payload, err := EncodeBlockDevelopment(candidate, s.rules.MaxPayloadSize)
    if err != nil {
        return err
    }
    for _, peer := range s.peers {
        if err := s.transport.Send(peer, Message{Type: MessageTypeBlock, Payload: payload}); err != nil {
            return err
        }
    }
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
