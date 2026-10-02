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

func TestConsensusSessionPumpCandidateBeforeProposal(t *testing.T) {
    nodeA, err := node.NewDevnet(storage.NewMemoryStore()); if err != nil { t.Fatal(err) }
    nodeB, err := node.NewDevnet(storage.NewMemoryStore()); if err != nil { t.Fatal(err) }
    keyA, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x71}, 32)); if err != nil { t.Fatal(err) }
    keyB, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x72}, 32)); if err != nil { t.Fatal(err) }
    signerA, err := crypto.NewEd25519Signer(keyA.PrivateKey); if err != nil { t.Fatal(err) }
    signerB, err := crypto.NewEd25519Signer(keyB.PrivateKey); if err != nil { t.Fatal(err) }
    va, vb := []byte("validator-a"), []byte("validator-b")
    validators, err := consensus.NewValidatorSet([][]byte{va,vb}); if err != nil { t.Fatal(err) }
    power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{{ValidatorID:va,Power:1},{ValidatorID:vb,Power:1}}); if err != nil { t.Fatal(err) }
    authority, err := consensus.NewStaticValidatorAuthority(map[string][]byte{string(va):keyA.PublicKey,string(vb):keyB.PublicKey}); if err != nil { t.Fatal(err) }
    stateA, err := consensus.NewRoundState(devnet.ProtocolVersion,devnet.ChainID,0,nodeA.Head.Header.Height); if err != nil { t.Fatal(err) }
    stateB, err := consensus.NewRoundState(devnet.ProtocolVersion,devnet.ChainID,0,nodeB.Head.Header.Height); if err != nil { t.Fatal(err) }
    rules, err := nodeA.Config.BlockRules(nil); if err != nil { t.Fatal(err) }
    ctxA := consensus.BlockProductionContext{State:stateA,PreviousHash:nodeA.HeadHash,Proposer:va}
    candidate, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{Context:ctxA,Timestamp:nodeA.Head.Header.Timestamp+1,Transactions:[]any{},Rules:rules},nodeA.State); if err != nil { t.Fatal(err) }
    ta,tb := NewInMemoryTransport(PeerID("a"),65536),NewInMemoryTransport(PeerID("b"),65536)
    if err:=ta.Connect("b",tb);err!=nil{t.Fatal(err)}; if err:=tb.Connect("a",ta);err!=nil{t.Fatal(err)}
    engineA:=newPumpSessionEngine(t,stateA,va,signerA,validators,power,authority)
    engineB:=newSessionEngine(t,stateB,vb,signerB,validators,power,authority)
    vr:=consensus.ValidationRules{ProtocolVersion:devnet.ProtocolVersion,ChainID:devnet.ChainID,MaxPayloadSize:65536,RequireSender:true,RequireSignature:true}
    ctxB:=consensus.BlockProductionContext{State:stateB,PreviousHash:nodeB.HeadHash,Proposer:va}
    sa,err:=NewConsensusSession(nodeA,engineA,ta,vr,[]PeerID{"b"},ctxA,validators,power,authority,pumpSenderResolver{key:keyA.PublicKey});if err!=nil{t.Fatal(err)}
    sb,err:=NewConsensusSession(nodeB,engineB,tb,vr,[]PeerID{"a"},ctxB,validators,power,authority,sessionSenderResolver{key:keyB.PublicKey});if err!=nil{t.Fatal(err)}
    if err:=sa.StartProposal(candidate);err!=nil{t.Fatal(err)}
    if _,err:=sb.PumpOnce();err!=nil{t.Fatal(err)} // candidate
    if _,err:=sb.PumpOnce();err!=nil{t.Fatal(err)} // proposal + local prevote
    if _,err:=sb.PumpOnce();err!=nil{t.Fatal(err)} // remote prevote -> local precommit
    if _,err:=sa.PumpOnce();err!=nil{t.Fatal(err)} // remote prevote -> local precommit
    if _,err:=sa.PumpOnce();err!=nil{t.Fatal(err)} // remote precommit -> finality
    if _,err:=sb.PumpOnce();err!=nil{t.Fatal(err)} // remote precommit -> finality
    if nodeA.HeadHash!=nodeB.HeadHash || nodeA.Head.Header.Height!=1 || nodeB.Head.Header.Height!=1 { t.Fatal("event pump did not converge canonical heads") }
    _=time.Second
}

type sessionSenderResolver struct{ key []byte }
func (r sessionSenderResolver) PublicKeyForSender([]byte)([]byte,error){return append([]byte(nil),r.key...),nil}
func newSessionEngine(t *testing.T,state consensus.RoundState,v []byte,signer crypto.Signer,validators consensus.ValidatorSet,power consensus.VotingPowerSet,authority consensus.StaticValidatorAuthority)*consensus.ConsensusEngine{
 t.Helper(); rt,err:=consensus.NewValidatorRuntime(consensus.RuntimeConfig{Rules:consensus.ValidationRules{ProtocolVersion:state.ProtocolVersion,ChainID:state.ChainID,RequireSender:true,RequireSignature:true},State:state,Validators:validators,VotingPower:power,Threshold:consensus.QuorumThreshold{Numerator:2,Denominator:3},Proposer:consensus.RoundRobinProposer{}});if err!=nil{t.Fatal(err)}
 e,err:=consensus.NewConsensusEngine(rt,authority,v,signer,consensus.TimeoutPolicy{Proposal:time.Second,Prevote:time.Second,Precommit:time.Second});if err!=nil{t.Fatal(err)};return e
}
