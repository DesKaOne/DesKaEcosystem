package consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
 )

const persistenceVectorVersion uint16 = 1

type persistenceVector struct {
	Version uint16 `json:"version"`
	ProtocolVersion types.ProtocolVersion `json:"protocol_version"`
	ChainID types.ChainID `json:"chain_id"`
	Epoch uint64 `json:"epoch"`
	Height types.Height `json:"height"`
	Round uint64 `json:"round"`
	Phase Phase `json:"phase"`
	Validators []persistenceValidator `json:"validators"`
	Threshold QuorumThreshold `json:"threshold"`
	ProposerPolicy string `json:"proposer_policy"`
}

type persistenceValidator struct {
	ID []byte `json:"id"`
	PublicKey []byte `json:"public_key"`
	VotingPower uint64 `json:"voting_power"`
}

func persistenceVectorFixture() persistenceVector {
	return persistenceVector{Version:persistenceVectorVersion, ProtocolVersion:1, ChainID:1001, Epoch:7, Height:42, Round:3, Phase:PhasePrecommit, Validators:[]persistenceValidator{{ID:[]byte("validator-a"), PublicKey:[]byte("pubkey-a"), VotingPower:3},{ID:[]byte("validator-b"), PublicKey:[]byte("pubkey-b"), VotingPower:2}}, Threshold:QuorumThreshold{Numerator:2, Denominator:3}, ProposerPolicy:"round-robin-v0-dev"}
}

func TestConsensusPersistenceVectorDeterministic(t *testing.T) {
	vector := persistenceVectorFixture()
	first, err := json.Marshal(vector); if err != nil { t.Fatal(err) }
	second, err := json.Marshal(vector); if err != nil { t.Fatal(err) }
	if !bytes.Equal(first, second) { t.Fatal("same semantic persistence state produced different bytes") }
	sum := sha256.Sum256(first)
	const wantDigest = "REPLACE_ME"
	if got := hex.EncodeToString(sum[:]); got != wantDigest { t.Fatalf("vector digest = %s, want %s; serialized=%s", got, wantDigest, first) }
}

func TestConsensusPersistenceVectorRoundTripSemanticState(t *testing.T) {
	vector := persistenceVectorFixture()
	encoded, err := json.Marshal(vector); if err != nil { t.Fatal(err) }
	var restored persistenceVector; if err := json.Unmarshal(encoded, &restored); err != nil { t.Fatal(err) }
	left, _ := json.Marshal(vector); right, _ := json.Marshal(restored)
	if !bytes.Equal(left, right) { t.Fatalf("semantic state changed after round-trip: %s != %s", left, right) }
}