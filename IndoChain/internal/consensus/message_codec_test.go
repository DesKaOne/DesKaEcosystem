package consensus

import (
    "bytes"
    "errors"
    "testing"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func TestConsensusMessageCodecRoundTrip(t *testing.T) {
    m := testConsensusMessage()
    rules := testConsensusRules()
    keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x42}, 32))
    if err != nil { t.Fatal(err) }
    signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
    if err != nil { t.Fatal(err) }
    m, err = m.Sign(signer)
    if err != nil { t.Fatal(err) }

    encoded, err := EncodeMessage(m, rules)
    if err != nil { t.Fatal(err) }
    decoded, err := DecodeMessage(encoded, rules)
    if err != nil { t.Fatal(err) }
    if !bytes.Equal(decoded.Sender, m.Sender) || !bytes.Equal(decoded.Payload, m.Payload) || !bytes.Equal(decoded.Signature, m.Signature) ||
        decoded.ProtocolVersion != m.ProtocolVersion || decoded.ChainID != m.ChainID || decoded.Epoch != m.Epoch ||
        decoded.Height != m.Height || decoded.Round != m.Round || decoded.Type != m.Type {
        t.Fatalf("decoded message = %+v, want %+v", decoded, m)
    }
}

func TestConsensusMessageCodecRejectsTrailingData(t *testing.T) {
    m := testConsensusMessage()
    rules := ValidationRules{ProtocolVersion: 1, ChainID: 1001}
    encoded, err := EncodeMessage(m, rules)
    if err != nil { t.Fatal(err) }
    encoded = append(encoded, 0)
    if _, err := DecodeMessage(encoded, rules); !errors.Is(err, ErrConsensusMessageEncoding) {
        t.Fatalf("error = %v, want %v", err, ErrConsensusMessageEncoding)
    }
}
