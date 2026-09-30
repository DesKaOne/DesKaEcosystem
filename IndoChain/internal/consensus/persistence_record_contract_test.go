package consensus

import (
	"bytes"
	"testing"
)

func persistenceTestContext() PersistenceContext {
	return PersistenceContext{
		ProtocolVersion: 1,
		ChainID: 1001,
		Epoch: 7,
		Height: 42,
		Round: 3,
		Phase: uint8(PhasePrecommit),
		ValidatorDigest: [32]byte{1, 2, 3},
		VotingPowerDigest: [32]byte{4, 5, 6},
		ThresholdNumerator: 2,
		ThresholdDenominator: 3,
		ProposerPolicy: "round-robin-v0-dev",
		ProposerPolicyVersion: "1",
	}
}

func TestPersistenceRecordEnvelopeDeterministicChecksum(t *testing.T) {
	context := persistenceTestContext()
	digest := PersistenceContextDigest(context)
	first, err := NewPersistenceRecord(PersistenceRecordTypeWAL, 1, digest, []byte("round-state"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewPersistenceRecord(PersistenceRecordTypeWAL, 1, digest, []byte("round-state"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Payload, second.Payload) || first.Checksum != second.Checksum {
		t.Fatal("same record semantics produced different checksum")
	}
	const want = "f346189c12f475b5e9546f0e4ea8168add37c71d99bb69760944e748a313b5d2"
	if got := PersistenceChecksumHex(first.Checksum); got != want {
		t.Fatalf("checksum = %s, want %s", got, want)
	}
}

func TestPersistenceRecordCrashBoundaryMatrix(t *testing.T) {
	context := persistenceTestContext()
	digest := PersistenceContextDigest(context)
	record1, err := NewPersistenceRecord(PersistenceRecordTypeWAL, 1, digest, []byte("one"))
	if err != nil { t.Fatal(err) }
	record2, err := NewPersistenceRecord(PersistenceRecordTypeWAL, 2, digest, []byte("two"))
	if err != nil { t.Fatal(err) }

	cases := []struct {
		name string
		record PersistenceRecordEnvelope
		previous uint64
		wantErr error
	}{
		{"valid append", record1, 0, nil},
		{"sequence gap", record2, 0, ErrPersistenceSequence},
		{"duplicate sequence", record1, 1, ErrPersistenceSequence},
		{"context mismatch", func() PersistenceRecordEnvelope {
			r := record1
			r.ContextDigest[0] ^= 0xff
			return r
		}(), 0, ErrPersistenceContextMismatch},
		{"checksum corruption", func() PersistenceRecordEnvelope {
			r := record1
			r.Payload = append([]byte(nil), record1.Payload...)
			r.Payload[0] ^= 0xff
			return r
		}(), 0, ErrPersistenceChecksum},
		{"format downgrade", func() PersistenceRecordEnvelope {
			r := record1
			r.FormatVersion = 0
			return r
		}(), 0, ErrPersistenceRecordVersion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.record.Validate(digest, tc.previous)
			if tc.wantErr == nil {
				if err != nil { t.Fatalf("Validate() error = %v", err) }
				return
			}
			if err != tc.wantErr { t.Fatalf("Validate() error = %v, want %v", err, tc.wantErr) }
		})
	}

	// Crash cut before the record is fully appended leaves the last durable
	// sequence unchanged; the partial bytes must not be treated as a record.
	if err := record2.Validate(digest, 1); err != nil {
		t.Fatalf("complete next record should validate after prior durable sequence: %v", err)
	}
}

func TestPersistenceSnapshotAndWALOrderingContract(t *testing.T) {
	context := persistenceTestContext()
	digest := PersistenceContextDigest(context)
	snapshot, err := NewPersistenceRecord(PersistenceRecordTypeSnapshot, 10, digest, []byte("snapshot-at-10"))
	if err != nil { t.Fatal(err) }
	wal11, err := NewPersistenceRecord(PersistenceRecordTypeWAL, 11, digest, []byte("wal-11"))
	if err != nil { t.Fatal(err) }
	wal12, err := NewPersistenceRecord(PersistenceRecordTypeWAL, 12, digest, []byte("wal-12"))
	if err != nil { t.Fatal(err) }

	if err := snapshot.Validate(digest, 0); err != nil { t.Fatal(err) }
	if err := wal11.Validate(digest, snapshot.Sequence); err != nil { t.Fatal(err) }
	if err := wal12.Validate(digest, wal11.Sequence); err != nil { t.Fatal(err) }
}

func TestPersistenceContextDigestBindsRecoveryContext(t *testing.T) {
	context := persistenceTestContext()
	original := PersistenceContextDigest(context)
	context.Round++
	changed := PersistenceContextDigest(context)
	if original == changed {
		t.Fatal("changing recovery context must change context digest")
	}
}
