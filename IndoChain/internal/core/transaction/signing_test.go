package transaction

import (
    "bytes"
    "testing"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestTransactionSignVerify(t *testing.T) {
    signer, err := crypto.NewEd25519Signer(bytes.Repeat([]byte{0x42}, 64))
    if err != nil { t.Fatal(err) }
    tx := Transaction{Version: 1, ChainID: 1001, Nonce: 7, Sender: types.Address{1,2,3}, Recipient: types.Address{4,5,6}, Value: 1000, GasLimit: 21000, Data: []byte("hello")}
    signature, err := Sign(tx, signer)
    if err != nil { t.Fatal(err) }
    ok, err := VerifySignature(tx, signature, signer.PublicKey())
    if err != nil { t.Fatal(err) }
    if !ok { t.Fatal("signature verification failed") }
    tx.Value++
    ok, err = VerifySignature(tx, signature, signer.PublicKey())
    if err != nil { t.Fatal(err) }
    if ok { t.Fatal("modified transaction unexpectedly verified") }
}

func TestDomainSigningBytesExcludeSignature(t *testing.T) {
    tx := Transaction{Version: 1, ChainID: 1001, Nonce: 1, Sender: types.Address{1}, Recipient: types.Address{2}, Value: 10, GasLimit: 1000}
    a, err := SigningBytesWithDomain(tx)
    if err != nil { t.Fatal(err) }
    tx.Signature = []byte{9,9,9}
    b, err := SigningBytesWithDomain(tx)
    if err != nil { t.Fatal(err) }
    if !bytes.Equal(a, b) { t.Fatal("signing bytes changed when signature changed") }
}
