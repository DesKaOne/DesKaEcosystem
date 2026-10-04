package p2p

import (
    "bytes"
    "testing"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func TestInMemoryTransportMultiNodeConsensusExchange(t *testing.T) {
    nodeA := NewInMemoryTransport(PeerID("node-a"), 4096)
    nodeB := NewInMemoryTransport(PeerID("node-b"), 4096)

    if err := nodeA.Connect(PeerID("node-b"), nodeB); err != nil {
        t.Fatal(err)
    }
    if err := nodeB.Connect(PeerID("node-a"), nodeA); err != nil {
        t.Fatal(err)
    }

    keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x42}, 32))
    if err != nil {
        t.Fatal(err)
    }
    signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
    if err != nil {
        t.Fatal(err)
    }

    rules := consensus.ValidationRules{
        ProtocolVersion:  1,
        ChainID:          1001,
        MaxPayloadSize:   64,
        RequireSender:    true,
        RequireSignature: true,
    }

    proposal := consensus.Message{
        ProtocolVersion: 1,
        ChainID:         1001,
        Epoch:           2,
        Height:          7,
        Round:           3,
        Sender:          []byte{1, 2, 3},
        Type:            consensus.MessageTypeProposal,
        Payload:         []byte{9, 8, 7},
    }
    proposal, err = proposal.Sign(signer)
    if err != nil {
        t.Fatal(err)
    }

    if err := nodeA.SendConsensus(PeerID("node-b"), proposal, rules); err != nil {
        t.Fatal(err)
    }

    from, receivedProposal, err := nodeB.ReceiveConsensus(rules)
    if err != nil {
        t.Fatal(err)
    }
    if from != PeerID("node-a") {
        t.Fatalf("proposal from = %q, want node-a", from)
    }
    if receivedProposal.Type != consensus.MessageTypeProposal ||
        receivedProposal.Height != proposal.Height ||
        !bytes.Equal(receivedProposal.Payload, proposal.Payload) ||
        !bytes.Equal(receivedProposal.Signature, proposal.Signature) {
        t.Fatalf("received proposal = %+v, want %+v", receivedProposal, proposal)
    }

    vote := consensus.Message{
        ProtocolVersion: 1,
        ChainID:         1001,
        Epoch:           2,
        Height:          7,
        Round:           3,
        Sender:          []byte{4, 5, 6},
        Type:            consensus.MessageTypeVote,
        Payload:         proposal.Payload,
    }
    vote, err = vote.Sign(signer)
    if err != nil {
        t.Fatal(err)
    }

    if err := nodeB.SendConsensus(PeerID("node-a"), vote, rules); err != nil {
        t.Fatal(err)
    }

    from, receivedVote, err := nodeA.ReceiveConsensus(rules)
    if err != nil {
        t.Fatal(err)
    }
    if from != PeerID("node-b") {
        t.Fatalf("vote from = %q, want node-b", from)
    }
    if receivedVote.Type != consensus.MessageTypeVote ||
        receivedVote.Height != vote.Height ||
        !bytes.Equal(receivedVote.Payload, vote.Payload) ||
        !bytes.Equal(receivedVote.Signature, vote.Signature) {
        t.Fatalf("received vote = %+v, want %+v", receivedVote, vote)
    }
}
