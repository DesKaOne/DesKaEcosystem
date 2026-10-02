package node

import (
    "bytes"
    "testing"
    "time"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/p2p"
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
        State: state,
        Validators: validators,
        VotingPower: power,
        Threshold: consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
        Proposer: consensus.RoundRobinProposer{},
    })
    if err != nil { t.Fatal(err) }
    engine, err := consensus.NewConsensusEngine(
        runtime,
        authority,
        validator,
        signer,
        consensus.TimeoutPolicy{Proposal: time.Second, Prevote: time.Second, Precommit: time.Second},
    )
    if err != nil { t.Fatal(err) }
    return engine
}

func TestConsensusSessionMultiNodeCommitExactlyOnce(t *testing.T) {
    storeA := storage.NewMemoryStore()
    storeB := storage.NewMemoryStore()
    nodeA, err := NewDevnet(storeA)
    if err != nil { t.Fatal(err) }
    nodeB, err := NewDevnet(storeB)
    if err != nil { t.Fatal(err) }

    keyA, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x51}, 32))
    if err != nil { t.Fatal(err) }
    keyB, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x52}, 32))
    if err != nil { t.Fatal(err) }
    signerA, err := crypto.NewEd25519Signer(keyA.PrivateKey)
    if err != nil { t.Fatal(err) }
    signerB, err := crypto.NewEd25519Signer(keyB.PrivateKey)
    if err != nil { t.Fatal(err) }

    validatorA := []byte("validator-a")
    validatorB := []byte("validator-b")
    validators, err := consensus.NewValidatorSet([][]byte{validatorA, validatorB})
    if err != nil { t.Fatal(err) }
    power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{
        {ValidatorID: validatorA, Power: 1},
        {ValidatorID: validatorB, Power: 1},
    })
    if err != nil { t.Fatal(err) }
    authority, err := consensus.NewStaticValidatorAuthority(map[string][]byte{
        string(validatorA): keyA.PublicKey,
        string(validatorB): keyB.PublicKey,
    })
    if err != nil { t.Fatal(err) }

    stateA, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, nodeA.Head.Header.Height)
    if err != nil { t.Fatal(err) }
    stateB, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, nodeB.Head.Header.Height)
    if err != nil { t.Fatal(err) }

    ctx := consensus.BlockProductionContext{
        State: stateA,
        PreviousHash: nodeA.HeadHash,
        Proposer: append([]byte(nil), validatorA...),
    }
    rules, err := nodeA.Config.BlockRules(nil)
    if err != nil { t.Fatal(err) }
    candidate, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
        Context: ctx,
        Timestamp: nodeA.Head.Header.Timestamp + 1,
        Transactions: []any{},
        Rules: rules,
    }, nodeA.State)
    if err != nil { t.Fatal(err) }

    transportA := p2p.NewInMemoryTransport(p2p.PeerID("node-a"), 65536)
    transportB := p2p.NewInMemoryTransport(p2p.PeerID("node-b"), 65536)
    if err := transportA.Connect(p2p.PeerID("node-b"), transportB); err != nil { t.Fatal(err) }
    if err := transportB.Connect(p2p.PeerID("node-a"), transportA); err != nil { t.Fatal(err) }

    engineA := newSessionEngine(t, stateA, validatorA, signerA, validators, power, authority)
    engineB := newSessionEngine(t, stateB, validatorB, signerB, validators, power, authority)

    messageRules := consensus.ValidationRules{
        ProtocolVersion: devnet.ProtocolVersion,
        ChainID: devnet.ChainID,
        MaxPayloadSize: 65536,
        RequireSender: true,
        RequireSignature: true,
    }
    sessionA, err := NewConsensusSession(
        nodeA, engineA, transportA, messageRules, []p2p.PeerID{"node-b"},
        ctx, validators, power, authority, sessionSenderResolver{key: keyA.PublicKey},
    )
    if err != nil { t.Fatal(err) }

    ctxB := consensus.BlockProductionContext{
        State: stateB,
        PreviousHash: nodeB.HeadHash,
        Proposer: append([]byte(nil), validatorA...),
    }
    sessionB, err := NewConsensusSession(
        nodeB, engineB, transportB, messageRules, []p2p.PeerID{"node-a"},
        ctxB, validators, power, authority, sessionSenderResolver{key: keyB.PublicKey},
    )
    if err != nil { t.Fatal(err) }

    if err := sessionA.StartProposal(candidate); err != nil {
        t.Fatalf("start proposal: %v", err)
    }

    if _, err := sessionB.ReceiveAndProcess(&candidate); err != nil {
        t.Fatalf("node B proposal: %v", err)
    }
    if _, err := sessionB.ReceiveAndProcess(nil); err != nil {
        t.Fatalf("node B remote prevote: %v", err)
    }
    if _, err := sessionA.ReceiveAndProcess(nil); err != nil {
        t.Fatalf("node A remote prevote: %v", err)
    }
    if _, err := sessionA.ReceiveAndProcess(nil); err != nil {
        t.Fatalf("node A remote precommit: %v", err)
    }
    if _, err := sessionB.ReceiveAndProcess(nil); err != nil {
        t.Fatalf("node B remote precommit: %v", err)
    }

    if nodeA.Head.Header.Height != 1 || nodeB.Head.Header.Height != 1 {
        t.Fatalf("heads did not converge: A=%d B=%d", nodeA.Head.Header.Height, nodeB.Head.Header.Height)
    }
    if nodeA.HeadHash != nodeB.HeadHash {
        t.Fatalf("head hashes diverged: A=%x B=%x", nodeA.HeadHash, nodeB.HeadHash)
    }

    committedHash := nodeA.HeadHash
    if err := sessionA.commitIfFinalized(); err != nil {
        t.Fatalf("duplicate commit path returned error: %v", err)
    }
    if nodeA.HeadHash != committedHash || nodeA.Head.Header.Height != 1 {
        t.Fatal("duplicate commit path changed canonical head")
    }

    storedA, hashA, err := storeA.Head()
    if err != nil { t.Fatal(err) }
    storedB, hashB, err := storeB.Head()
    if err != nil { t.Fatal(err) }
    if storedA.Header.Height != 1 || storedB.Header.Height != 1 || hashA != hashB {
        t.Fatal("durable heads did not converge")
    }
}

func TestConsensusSessionRejectsMissingProposalCandidate(t *testing.T) {
    store := storage.NewMemoryStore()
    n, err := NewDevnet(store)
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
    transport := p2p.NewInMemoryTransport(p2p.PeerID("node-a"), 4096)
    rules := consensus.ValidationRules{
        ProtocolVersion: devnet.ProtocolVersion, ChainID: devnet.ChainID,
        RequireSender: true, RequireSignature: true,
    }
    ctx := consensus.BlockProductionContext{State: state, PreviousHash: n.HeadHash, Proposer: validator}
    session, err := NewConsensusSession(
        n, engine, transport, rules, []p2p.PeerID{"node-b"}, ctx, validators, power,
        authority, sessionSenderResolver{key: key.PublicKey},
    )
    if err != nil { t.Fatal(err) }

    candidate := block.Block{}
    proposal, err := engine.BuildProposalMessage(candidate)
    if err == nil { t.Fatalf("unexpected proposal from invalid candidate: %+v", proposal) }
    if err := session.HandlePeerMessage("node-b", proposal, nil); err == nil {
        t.Fatal("missing proposal candidate unexpectedly accepted")
    }
}
