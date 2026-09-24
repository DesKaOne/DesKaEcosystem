package p2p

import (
    "bytes"
    "testing"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func TestInMemoryTransportRoutesConsensusMessage(t *testing.T) {
    a := NewInMemoryTransport(PeerID("node-a"), 4096)
    b := NewInMemoryTransport(PeerID("node-b"), 4096)
    if err := a.Connect(PeerID("node-b"), b); err != nil { t.Fatal(err) }

    msg := consensus.Message{
        ProtocolVersion: 1, ChainID: 1001, Epoch: 2, Height: 7, Round: 3,
        Sender: []byte{1, 2, 3}, Type: consensus.MessageTypeVote, Payload: []byte{9, 8, 7},
    }
    keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x42}, 32))
    if err != nil { t.Fatal(err) }
    signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
    if err != nil { t.Fatal(err) }
    msg, err = msg.Sign(signer)
    if err != nil { t.Fatal(err) }

    rules := consensus.ValidationRules{ProtocolVersion: 1, ChainID: 1001, MaxPayloadSize: 16, RequireSender: true, RequireSignature: true}
    if err := a.SendConsensus(PeerID("node-b"), msg, rules); err != nil { t.Fatal(err) }

    from, got, err := b.ReceiveConsensus(rules)
    if err != nil { t.Fatal(err) }
    if from != PeerID("node-a") { t.Fatalf("from = %q, want node-a", from) }
    if got.Type != msg.Type || got.ChainID != msg.ChainID || !bytes.Equal(got.Payload, msg.Payload) || !bytes.Equal(got.Signature, msg.Signature) {
        t.Fatalf("got = %+v, want %+v", got, msg)
    }
}

func TestInMemoryTransportRejectsConsensusPayloadThatExceedsTransportLimit(t *testing.T) {
    a := NewInMemoryTransport(PeerID("node-a"), 16)
    b := NewInMemoryTransport(PeerID("node-b"), 16)
    if err := a.Connect(PeerID("node-b"), b); err != nil { t.Fatal(err) }
    msg := consensus.Message{ProtocolVersion: 1, ChainID: 1001, Sender: []byte{1}, Type: consensus.MessageTypeVote, Payload: []byte{1, 2, 3}}
    rules := consensus.ValidationRules{ProtocolVersion: 1, ChainID: 1001, MaxPayloadSize: 16, RequireSender: true}
    if err := a.SendConsensus(PeerID("node-b"), msg, rules); err == nil {
        t.Fatal("expected transport payload limit error")
    }
}
