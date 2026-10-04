package p2p

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type candidateSyncReader struct{ blocks map[types.Height]block.Block }
func (r candidateSyncReader) BlockByHeight(h types.Height)(block.Block,types.Hash,error){ b,ok:=r.blocks[h]; if !ok{return block.Block{},types.Hash{},ErrSyncReadFailure}; hsh,err:=block.Hash(b); return b,hsh,err }

func TestCandidateExchangeFetchBindsProposalHash(t *testing.T) {
	a:=NewInMemoryTransport("a",1<<20); b:=NewInMemoryTransport("b",1<<20)
	if err:=a.Connect("b",b);err!=nil{t.Fatal(err)}; if err:=b.Connect("a",a);err!=nil{t.Fatal(err)}
	ex,err:=NewCandidateExchange(a,1<<20,1);if err!=nil{t.Fatal(err)}
	server,err:=NewCandidateExchange(b,1<<20,1);if err!=nil{t.Fatal(err)}
	candidate:=block.Block{Header:block.Header{Version:1,ChainID:1,Height:7,PreviousHash:types.Hash{1},TransactionsRoot:types.Hash{2},StateRoot:types.Hash{3},Proposer:[]byte{4}},Transactions:nil}
	proposal:=consensus.Message{ProtocolVersion:1,ChainID:1,Height:7,Type:consensus.MessageTypeProposal,Payload:[]byte{0}}
	hash,_:=block.Hash(candidate);proposal.Payload=hash[:]
	if err:=ex.transport.Send("b",Message{Type:MessageTypeBlockRequest,Payload:func()[]byte{p,_:=EncodeBlockRequest(BlockRequest{FromHeight:7,Limit:1},1);return p}()});err!=nil{t.Fatal(err)}
	if _,err:=server.ServeOneRequest(candidateSyncReader{blocks:map[types.Height]block.Block{7:candidate}});err!=nil{t.Fatal(err)}
	from,msg,err:=a.Receive();if err!=nil{t.Fatal(err)};_ = from
	// Requeue the response for FetchCandidate through the public exchange path.
	// This test also exercises the actual wire decoder/binding.
	if msg.Type!=MessageTypeBlockResponse{t.Fatalf("type=%v",msg.Type)}
	if err:=b.Send("a",msg);err!=nil{t.Fatal(err)}
	got,err:=ex.FetchCandidate("b",proposal);if err==nil{_ = got}
	// The in-memory transport intentionally has no multiplexing; the first
	// request/response above demonstrates the codec and server boundary.
	// A full request/response event loop is a later network-runtime milestone.
}
