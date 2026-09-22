package transaction

import (
    "testing"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestHashChangesWithSignedFields(t *testing.T) {
    tx := Transaction{
        Version: 1, ChainID: 1001, Nonce: 1,
        Sender: types.Address{1}, Recipient: types.Address{2},
        Value: 10, GasLimit: 21000, Data: []byte("a"),
        Signature: []byte{3},
    }

    original := Hash(tx)

    tx.Nonce++
    if Hash(tx) == original {
        t.Fatal("hash did not change when nonce changed")
    }

    tx.Nonce--
    tx.Signature[0]++
    if Hash(tx) == original {
        t.Fatal("hash did not change when signature changed")
    }
}

func TestHashIncludesAllEncodedFields(t *testing.T) {
    base := Transaction{
        Version: 1, ChainID: 1001, Nonce: 1,
        Sender: types.Address{1}, Recipient: types.Address{2},
        Value: 10, GasLimit: 21000, Data: []byte("a"),
        Signature: []byte{3},
    }

    fields := []func(*Transaction){
        func(tx *Transaction) { tx.Version++ },
        func(tx *Transaction) { tx.ChainID++ },
        func(tx *Transaction) { tx.Nonce++ },
        func(tx *Transaction) { tx.Sender[0]++ },
        func(tx *Transaction) { tx.Recipient[0]++ },
        func(tx *Transaction) { tx.Value++ },
        func(tx *Transaction) { tx.GasLimit++ },
        func(tx *Transaction) { tx.Data[0]++ },
        func(tx *Transaction) { tx.Signature[0]++ },
    }

    original := Hash(base)
    for i, mutate := range fields {
        tx := base
        tx.Sender = append([]byte(nil), base.Sender...)
        tx.Recipient = append([]byte(nil), base.Recipient...)
        tx.Data = append([]byte(nil), base.Data...)
        tx.Signature = append([]byte(nil), base.Signature...)
        mutate(&tx)
        if Hash(tx) == original {
            t.Fatalf("hash did not change for field %d", i)
        }
    }
}
