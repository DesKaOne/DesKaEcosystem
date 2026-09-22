package transaction

import (
	"bytes"
	"testing"
)

func sampleTransaction() Transaction {
	return Transaction{
		Version:    1,
		ChainID:    1001,
		Nonce:      7,
		Sender:     []byte{1, 2, 3},
		Recipient:  []byte{4, 5, 6},
		Value:      42,
		GasLimit:   21000,
		Data:       []byte("indochain"),
		Signature:  []byte{9, 9, 9},
	}
}

func TestSigningBytesExcludeSignature(t *testing.T) {
	tx := sampleTransaction()

	signing, err := SigningBytes(tx)
	if err != nil {
		t.Fatal(err)
	}

	tx.Signature = []byte{8, 8, 8, 8}
	signing2, err := SigningBytes(tx)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(signing, signing2) {
		t.Fatal("signing bytes must not depend on signature")
	}
}

func TestSignedBytesDependOnSignature(t *testing.T) {
	tx := sampleTransaction()

	a, err := SignedBytes(tx)
	if err != nil {
		t.Fatal(err)
	}

	tx.Signature = []byte{1}
	b, err := SignedBytes(tx)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(a, b) {
		t.Fatal("signed bytes must include signature")
	}
}
