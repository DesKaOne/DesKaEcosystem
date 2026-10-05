package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestDevelopmentBlockCodecRoundTrip(t *testing.T) {
	candidate := block.Block{
		Header:block.Header{Version:1,ChainID:1,Height:7,Timestamp:123,PreviousHash:types.Hash{1},TransactionsRoot:types.Hash{2},StateRoot:types.Hash{3},Proposer:[]byte{4},ConsensusEvidence:[]byte{5}},
		Transactions:[]any{transaction.Transaction{Version:1,ChainID:1,Nonce:2,Sender:[]byte{6},Recipient:[]byte{7},Value:8,GasLimit:9,Data:[]byte{10},Signature:[]byte{11}}},
	}
	encoded,err:=EncodeBlockDevelopment(candidate,4096); if err!=nil{t.Fatal(err)}
	decoded,err:=DecodeBlockDevelopment(encoded,4096); if err!=nil{t.Fatal(err)}
	got,err:=block.Hash(decoded); if err!=nil{t.Fatal(err)}
	want,err:=block.Hash(candidate); if err!=nil{t.Fatal(err)}
	if got!=want{t.Fatalf("hash mismatch: got %s want %s",got,want)}
}
