package p2p

import (
    "bytes"
    "testing"
    "time"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
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
