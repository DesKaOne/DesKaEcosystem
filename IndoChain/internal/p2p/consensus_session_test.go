package p2p

import (
    "bytes"
    "testing"
    "time"
    "sync"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/node"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

type sessionSenderResolver struct{ key []byte }

func (r sessionSenderResolver) PublicKeyForSender([]byte) ([]byte, error) {
    return append([]byte(nil), r.key...), nil
}

func newSessionEngine(
    t *testing.T,
    state consensus.RoundState,
    validator []byte,
    signer crypto.Signer,
    validators consensus.ValidatorSet,
    power consensus.VotingPowerSet,
    authority consensus.StaticValidatorAuthority,
) *consensus.ConsensusEngine {
    t.Helper()
    runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
        Rules: consensus.ValidationRules{
            ProtocolVersion: state.ProtocolVersion,
            ChainID: state.ChainID,
            RequireSender: true,
            RequireSignature: true,
        },
        State: state, Validators: validators, VotingPower: power,
        Threshold: consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
        Proposer: consensus.RoundRobinProposer{},
    })
    if err != nil { t.Fatal(err) }
    engine, err := consensus.NewConsensusEngine(
        runtime, authority, validator, signer,
        consensus.TimeoutPolicy{Proposal: time.Second, Prevote: time.Second, Precommit: time.Second},
    )
    if err != nil { t.Fatal(err) }
    return engine
}

func TestConsensusSessionMultiNodeCommitExactlyOnce(t *testing.T) {
    nodeA, err := node.NewDevnet(storage.NewMemoryStore())
    if err != nil { t.Fatal(err) }
    nodeB, err := node.NewDevnet(storage.NewMemoryStore())
    if err != nil { t.Fatal(err) }

    keyA, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x51}, 32))
    if err != nil { t.Fatal(err) }
    keyB, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x52}, 32))
    if err != nil { t.Fatal(err) }
    signerA, err := crypto.NewEd25519Signer(keyA.PrivateKey)
    if err != nil { t.Fatal(err) }
    signerB, err := crypto.NewEd25519Signer(keyB.PrivateKey)
    if err != nil { t.Fatal(err) }

    validatorA, validatorB := []byte("validator-a"), []byte("validator-b")
    validators, err := consensus.NewValidatorSet([][]byte{validatorA, validatorB})
    if err != nil { t.Fatal(err) }
    power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{
        {ValidatorID: validatorA, Power: 1}, {ValidatorID: validatorB, Power: 1},
    })
    if err != nil { t.Fatal(err) }
    authority, err := consensus.NewStaticValidatorAuthority(map[string][]byte{
        string(validatorA): keyA.PublicKey, string(validatorB): keyB.PublicKey,
    })
    if err != nil { t.Fatal(err) }

    stateA, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, nodeA.Head.Header.Height)
    if err != nil { t.Fatal(err) }
    stateB, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, nodeB.Head.Header.Height)
    if err != nil { t.Fatal(err) }

    ctxA := consensus.BlockProductionContext{State: stateA, PreviousHash: nodeA.HeadHash, Proposer: validatorA}
    rules, err := nodeA.Config.BlockRules(nil)
    if err != nil { t.Fatal(err) }
    candidate, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
        Context: ctxA, Timestamp: nodeA.Head.Header.Timestamp + 1,
        Transactions: []any{}, Rules: rules,
    }, nodeA.State)
    if err != nil { t.Fatal(err) }

    transportA := NewInMemoryTransport(PeerID("node-a"), 65536)
    transportB := NewInMemoryTransport(PeerID("node-b"), 65536)
    if err := transportA.Connect(PeerID("node-b"), transportB); err != nil { t.Fatal(err) }
    if err := transportB.Connect(PeerID("node-a"), transportA); err != nil { t.Fatal(err) }

    engineA := newSessionEngine(t, stateA, validatorA, signerA, validators, power, authority)
    engineB := newSessionEngine(t, stateB, validatorB, signerB, validators, power, authority)
    messageRules := consensus.ValidationRules{
        ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID,
        MaxPayloadSize: 65536, RequireSender: true, RequireSignature: true,
    }

    sessionA, err := NewConsensusSession(
        nodeA, engineA, transportA, messageRules, []PeerID{"node-b"},
        ctxA, validators, power, authority, sessionSenderResolver{key: keyA.PublicKey},
    )
    if err != nil { t.Fatal(err) }

    ctxB := consensus.BlockProductionContext{State: stateB, PreviousHash: nodeB.HeadHash, Proposer: validatorA}
    sessionB, err := NewConsensusSession(
        nodeB, engineB, transportB, messageRules, []PeerID{"node-a"},
        ctxB, validators, power, authority, sessionSenderResolver{key: keyB.PublicKey},
    )
    if err != nil { t.Fatal(err) }

    if err := sessionA.StartProposal(candidate); err != nil { t.Fatalf("start proposal: %v", err) }
    if _, err := sessionB.ReceiveAndProcess(&candidate); err != nil { t.Fatalf("node B proposal: %v", err) }
    if _, err := sessionB.ReceiveAndProcess(nil); err != nil { t.Fatalf("node B remote prevote: %v", err) }
    if _, err := sessionA.ReceiveAndProcess(nil); err != nil { t.Fatalf("node A remote prevote: %v", err) }
    if _, err := sessionA.ReceiveAndProcess(nil); err != nil { t.Fatalf("node A remote precommit: %v", err) }
    if _, err := sessionB.ReceiveAndProcess(nil); err != nil { t.Fatalf("node B remote precommit: %v", err) }

    if nodeA.Head.Header.Height != 1 || nodeB.Head.Header.Height != 1 {
        t.Fatalf("heads did not converge: A=%d B=%d", nodeA.Head.Header.Height, nodeB.Head.Header.Height)
    }
    if nodeA.HeadHash != nodeB.HeadHash {
        t.Fatalf("head hashes diverged: A=%x B=%x", nodeA.HeadHash, nodeB.HeadHash)
    }
    committedHash := nodeA.HeadHash
    if err := sessionA.commitIfFinalized(); err != nil { t.Fatalf("duplicate commit path: %v", err) }
    if nodeA.HeadHash != committedHash || nodeA.Head.Header.Height != 1 {
        t.Fatal("duplicate commit path changed canonical head")
    }
}

func TestConsensusSessionRejectsMissingProposalCandidate(t *testing.T) {
    n, err := node.NewDevnet(storage.NewMemoryStore())
    if err != nil { t.Fatal(err) }
    state, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, n.Head.Header.Height)
    if err != nil { t.Fatal(err) }
    validator := []byte("validator-a")
    key, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x61}, 32))
    if err != nil { t.Fatal(err) }
    signer, err := crypto.NewEd25519Signer(key.PrivateKey)
    if err != nil { t.Fatal(err) }
    validators, err := consensus.NewValidatorSet([][]byte{validator})
    if err != nil { t.Fatal(err) }
    power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{{ValidatorID: validator, Power: 1}})
    if err != nil { t.Fatal(err) }
    authority, err := consensus.NewStaticValidatorAuthority(map[string][]byte{string(validator): key.PublicKey})
    if err != nil { t.Fatal(err) }
    engine := newSessionEngine(t, state, validator, signer, validators, power, authority)
    transport := NewInMemoryTransport(PeerID("node-a"), 4096)
    ctx := consensus.BlockProductionContext{State: state, PreviousHash: n.HeadHash, Proposer: validator}
    session, err := NewConsensusSession(
        n, engine, transport,
        consensus.ValidationRules{ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID, RequireSender: true, RequireSignature: true},
        []PeerID{"node-b"}, ctx, validators, power, authority,
        sessionSenderResolver{key: key.PublicKey},
    )
    if err != nil { t.Fatal(err) }
    if err := session.HandlePeerMessage("node-b", consensus.Message{Type: consensus.MessageTypeProposal}, nil); err != ErrConsensusSessionCandidateRequired {
        t.Fatalf("error = %v, want candidate-required", err)
    }
}


type sessionTestTimer struct {
    mu sync.Mutex
    stopped bool
    fireFn func()
}
func (t *sessionTestTimer) Stop() bool {
    t.mu.Lock(); defer t.mu.Unlock()
    active := !t.stopped
    t.stopped = true
    return active
}
func (t *sessionTestTimer) Fire() {
    t.mu.Lock()
    if t.stopped { t.mu.Unlock(); return }
    fn := t.fireFn
    t.stopped = true
    t.mu.Unlock()
    fn()
}
type sessionTestClock struct {
    mu sync.Mutex
    timers []*sessionTestTimer
}
func (c *sessionTestClock) AfterFunc(_ time.Duration, fn func()) consensus.ConsensusTimeoutTimer {
    timer := &sessionTestTimer{fireFn: fn}
    c.mu.Lock(); c.timers = append(c.timers, timer); c.mu.Unlock()
    return timer
}
func (c *sessionTestClock) Timer(i int) *sessionTestTimer {
    c.mu.Lock(); defer c.mu.Unlock()
    return c.timers[i]
}

func TestConsensusSessionTimeoutSchedulerLifecycle(t *testing.T) {
    n, err := node.NewDevnet(storage.NewMemoryStore())
    if err != nil { t.Fatal(err) }
    validator := []byte("validator-a")
    key, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x71}, 32))
    if err != nil { t.Fatal(err) }
    signer, err := crypto.NewEd25519Signer(key.PrivateKey)
    if err != nil { t.Fatal(err) }
    validators, err := consensus.NewValidatorSet([][]byte{validator})
    if err != nil { t.Fatal(err) }
    power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{{ValidatorID: validator, Power: 1}})
    if err != nil { t.Fatal(err) }
    authority, err := consensus.NewStaticValidatorAuthority(map[string][]byte{string(validator): key.PublicKey})
    if err != nil { t.Fatal(err) }
    state, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, n.Head.Header.Height)
    if err != nil { t.Fatal(err) }
    engine := newSessionEngine(t, state, validator, signer, validators, power, authority)
    transport := NewInMemoryTransport(PeerID("node-a"), 65536)
    ctx := consensus.BlockProductionContext{State: state, PreviousHash: n.HeadHash, Proposer: validator}
    clock := &sessionTestClock{}
    session, err := NewConsensusSessionWithScheduler(
        n, engine, transport,
        consensus.ValidationRules{ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID, MaxPayloadSize: 65536, RequireSender: true, RequireSignature: true},
        []PeerID{"node-b"}, ctx, validators, power, authority,
        sessionSenderResolver{key: key.PublicKey}, clock.AfterFunc,
    )
    if err != nil { t.Fatal(err) }
    if err := session.Start(); err != nil { t.Fatal(err) }
    if len(clock.timers) != 1 { t.Fatalf("timer count = %d, want 1", len(clock.timers)) }
    session.Stop()
    clock.Timer(0).Fire()
    if len(engine.Driver().TimeoutEvidence()) != 0 {
        t.Fatal("stopped scheduler delivered timeout evidence")
    }
}

func TestConsensusSessionTimeoutProducesAuthenticatedEvidence(t *testing.T) {
    n, err := node.NewDevnet(storage.NewMemoryStore())
    if err != nil { t.Fatal(err) }
    validator := []byte("validator-a")
    key, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x72}, 32))
    if err != nil { t.Fatal(err) }
    signer, err := crypto.NewEd25519Signer(key.PrivateKey)
    if err != nil { t.Fatal(err) }
    validators, err := consensus.NewValidatorSet([][]byte{validator})
    if err != nil { t.Fatal(err) }
    power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{{ValidatorID: validator, Power: 1}})
    if err != nil { t.Fatal(err) }
    authority, err := consensus.NewStaticValidatorAuthority(map[string][]byte{string(validator): key.PublicKey})
    if err != nil { t.Fatal(err) }
    state, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, n.Head.Header.Height)
    if err != nil { t.Fatal(err) }
    engine := newSessionEngine(t, state, validator, signer, validators, power, authority)
    transport := NewInMemoryTransport(PeerID("node-a"), 65536)
    ctx := consensus.BlockProductionContext{State: state, PreviousHash: n.HeadHash, Proposer: validator}
    clock := &sessionTestClock{}
    session, err := NewConsensusSessionWithScheduler(
        n, engine, transport,
        consensus.ValidationRules{ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID, MaxPayloadSize: 65536, RequireSender: true, RequireSignature: true},
        []PeerID{"node-b"}, ctx, validators, power, authority,
        sessionSenderResolver{key: key.PublicKey}, clock.AfterFunc,
    )
    if err != nil { t.Fatal(err) }
    if err := session.Start(); err != nil { t.Fatal(err) }
    clock.Timer(0).Fire()
    evidence := engine.Driver().TimeoutEvidence()
    if len(evidence) != 1 || evidence[0].Type != consensus.MessageTypeTimeout {
        t.Fatalf("timeout evidence = %+v", evidence)
    }
    if err := consensus.VerifyMessageSignature(evidence[0], key.PublicKey); err != nil {
        t.Fatalf("timeout evidence signature invalid: %v", err)
    }
    session.Stop()
}


type sessionBlockProducer struct {
    n *node.Node
}

func (p sessionBlockProducer) ProduceBlock(ctx consensus.BlockProductionContext) (block.Block, error) {
    rules, err := p.n.Config.BlockRules(nil)
    if err != nil {
        return block.Block{}, err
    }
    return consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
        Context: ctx,
        Timestamp: p.n.Head.Header.Timestamp + 1,
        Transactions: []any{},
        Rules: rules,
    }, p.n.State)
}

func TestConsensusSessionAutomaticRoundChangeProposalHandoff(t *testing.T) {
    n, err := node.NewDevnet(storage.NewMemoryStore())
    if err != nil { t.Fatal(err) }

    keyA, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x81}, 32))
    if err != nil { t.Fatal(err) }
    keyB, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x82}, 32))
    if err != nil { t.Fatal(err) }
    signerB, err := crypto.NewEd25519Signer(keyB.PrivateKey)
    if err != nil { t.Fatal(err) }

    validatorA, validatorB := []byte("validator-a"), []byte("validator-b")
    validators, err := consensus.NewValidatorSet([][]byte{validatorA, validatorB})
    if err != nil { t.Fatal(err) }
    power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{
        {ValidatorID: validatorA, Power: 1}, {ValidatorID: validatorB, Power: 1},
    })
    if err != nil { t.Fatal(err) }
    authority, err := consensus.NewStaticValidatorAuthority(map[string][]byte{
        string(validatorA): keyA.PublicKey, string(validatorB): keyB.PublicKey,
    })
    if err != nil { t.Fatal(err) }

    state, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, n.Head.Header.Height)
    if err != nil { t.Fatal(err) }
    engine := newSessionEngine(t, state, validatorB, signerB, validators, power, authority)

    transport := NewInMemoryTransport(PeerID("node-b"), 65536)
    ctx := consensus.BlockProductionContext{State: state, PreviousHash: n.HeadHash, Proposer: validatorA}
    session, err := NewConsensusSessionWithSchedulerAndProducer(
        n, engine, transport,
        consensus.ValidationRules{
            ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID,
            MaxPayloadSize: 65536, RequireSender: true, RequireSignature: true,
        },
        []PeerID{"node-a"}, ctx, validators, power, authority,
        sessionSenderResolver{key: keyB.PublicKey}, nil, sessionBlockProducer{n: n},
    )
    if err != nil { t.Fatal(err) }
    defer session.Stop()

    token, _, err := engine.ArmTimeout()
    if err != nil { t.Fatal(err) }
    if _, err := engine.HandleTimeout(token); err != nil { t.Fatal(err) }

    otherTokenEngine, err := newSessionTimeoutEngineForHandoff(state, validatorA, keyA.PrivateKey, validators, power, authority)
    if err != nil { t.Fatal(err) }
    otherToken, _, err := otherTokenEngine.ArmTimeout()
    if err != nil { t.Fatal(err) }
    remoteTimeout, err := otherTokenEngine.HandleTimeout(otherToken)
    if err != nil { t.Fatal(err) }

    if len(engine.Driver().TimeoutEvidence()) != 1 {
        t.Fatalf("local timeout evidence = %d, want 1", len(engine.Driver().TimeoutEvidence()))
    }
    if err := session.HandlePeerMessage("node-a", remoteTimeout, nil); err != nil {
        t.Fatalf("timeout handoff: %v", err)
    }

    if engine.Runtime().State().Round == 0 {
        if _, err := engine.TryAdvanceRound(); err != nil {
            t.Fatalf("round advance after peer timeout: %v; evidence=%d", err, len(engine.Driver().TimeoutEvidence()))
        }
    }
    if engine.Runtime().State().Round != 1 {
        t.Fatalf("round = %d, want 1", engine.Runtime().State().Round)
    }
    if engine.Runtime().State().Phase != consensus.PhasePrevote {
        t.Fatalf("phase = %v, want prevote after automatic proposal", engine.Runtime().State().Phase)
    }
    if session.candidate == nil {
        t.Fatal("automatic proposal candidate missing")
    }
    if !bytes.Equal(session.candidate.Header.Proposer, validatorB) {
        t.Fatalf("automatic candidate proposer = %q, want %q", session.candidate.Header.Proposer, validatorB)
    }
}

func newSessionTimeoutEngineForHandoff(
    state consensus.RoundState,
    validator []byte,
    privateKey []byte,
    validators consensus.ValidatorSet,
    power consensus.VotingPowerSet,
    authority consensus.StaticValidatorAuthority,
) (*consensus.ConsensusEngine, error) {
    signer, err := crypto.NewEd25519Signer(privateKey)
    if err != nil { return nil, err }
    runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
        Rules: consensus.ValidationRules{
            ProtocolVersion: state.ProtocolVersion,
            ChainID: state.ChainID,
            RequireSender: true,
            RequireSignature: true,
        },
        State: state, Validators: validators, VotingPower: power,
        Threshold: consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
        Proposer: consensus.RoundRobinProposer{},
    })
    if err != nil { return nil, err }
    return consensus.NewConsensusEngine(
        runtime, authority, validator, signer,
        consensus.TimeoutPolicy{Proposal: time.Second, Prevote: time.Second, Precommit: time.Second},
    )
}
