package consensus

import (
	"errors"
	"testing"
)

var (
	errInjectedBeforeAppend = errors.New("injected failure before append")
	errInjectedPartialWrite = errors.New("injected partial write")
)

type persistenceFaultMode uint8

const (
	persistenceFaultNone persistenceFaultMode = iota
	persistenceFaultBeforeAppend
	persistenceFaultPartialWrite
	persistenceFaultChecksum
	persistenceFaultContextMismatch
	persistenceFaultSequenceGap
)

type persistenceTestAdapter struct {
	contextDigest [32]byte
	records       []PersistenceRecordEnvelope
	fault         persistenceFaultMode
	nextSequence  uint64
}

func newPersistenceTestAdapter(contextDigest [32]byte) *persistenceTestAdapter {
	return &persistenceTestAdapter{
		contextDigest: contextDigest,
		nextSequence:  1,
	}
}

func (a *persistenceTestAdapter) append(recordType uint8, payload []byte) error {
	return a.appendAt(recordType, a.nextSequence, payload)
}

func (a *persistenceTestAdapter) appendAt(recordType uint8, sequence uint64, payload []byte) error {
	if a.fault == persistenceFaultBeforeAppend {
		return errInjectedBeforeAppend
	}

	contextDigest := a.contextDigest
	if a.fault == persistenceFaultContextMismatch {
		contextDigest[0] ^= 0xff
	}
	record, err := NewPersistenceRecord(recordType, sequence, contextDigest, payload)
	if err != nil {
		return err
	}
	if a.fault == persistenceFaultChecksum {
		record.Checksum[0] ^= 0xff
	}
	if a.fault == persistenceFaultSequenceGap {
		record.Sequence++
	}
	if a.fault == persistenceFaultPartialWrite {
		return errInjectedPartialWrite
	}

	a.records = append(a.records, record)
	a.nextSequence = record.Sequence + 1
	a.fault = persistenceFaultNone
	return nil
}

type persistenceRecoveredState struct {
	LastSequence uint64
	Payloads     [][]byte
}

func recoverPersistence(records []PersistenceRecordEnvelope, expectedContext [32]byte) (persistenceRecoveredState, error) {
	var recovered persistenceRecoveredState
	if len(records) == 0 {
		return recovered, nil
	}

	snapshotIndex := -1
	for i, record := range records {
		if record.RecordType == PersistenceRecordTypeSnapshot {
			snapshotIndex = i
		}
	}
	start := 0
	previousSequence := uint64(0)
	if snapshotIndex >= 0 {
		snapshot := records[snapshotIndex]
		if err := snapshot.Validate(expectedContext, 0); err != nil {
			return persistenceRecoveredState{}, err
		}
		recovered.Payloads = append(recovered.Payloads, append([]byte(nil), snapshot.Payload...))
		recovered.LastSequence = snapshot.Sequence
		previousSequence = snapshot.Sequence
		start = snapshotIndex + 1
	}

	for i := start; i < len(records); i++ {
		record := records[i]
		if record.RecordType == PersistenceRecordTypeSnapshot {
			return persistenceRecoveredState{}, ErrPersistenceSequence
		}
		if err := record.Validate(expectedContext, previousSequence); err != nil {
			return persistenceRecoveredState{}, err
		}
		recovered.Payloads = append(recovered.Payloads, append([]byte(nil), record.Payload...))
		recovered.LastSequence = record.Sequence
		previousSequence = record.Sequence
	}
	return recovered, nil
}

func publishPersistenceRecovery(target *persistenceRecoveredState, records []PersistenceRecordEnvelope, expectedContext [32]byte) error {
	candidate, err := recoverPersistence(records, expectedContext)
	if err != nil {
		return err
	}
	*target = candidate
	return nil
}

func TestPersistenceFailureInjectionBeforeAppendLeavesDurableBoundaryUnchanged(t *testing.T) {
	context := persistenceTestContext()
	adapter := newPersistenceTestAdapter(PersistenceContextDigest(context))

	if err := adapter.append(PersistenceRecordTypeWAL, []byte("one")); err != nil {
		t.Fatal(err)
	}
	before := append([]PersistenceRecordEnvelope(nil), adapter.records...)
	adapter.fault = persistenceFaultBeforeAppend

	if err := adapter.append(PersistenceRecordTypeWAL, []byte("two")); !errors.Is(err, errInjectedBeforeAppend) {
		t.Fatalf("append error = %v, want injected failure", err)
	}
	if len(adapter.records) != len(before) || adapter.records[0].Sequence != before[0].Sequence {
		t.Fatal("durable boundary changed after failure before append")
	}
}

func TestPersistenceFailureInjectionPartialWriteLeavesNoPartialRecord(t *testing.T) {
	context := persistenceTestContext()
	adapter := newPersistenceTestAdapter(PersistenceContextDigest(context))
	if err := adapter.append(PersistenceRecordTypeWAL, []byte("one")); err != nil {
		t.Fatal(err)
	}
	adapter.fault = persistenceFaultPartialWrite
	if err := adapter.append(PersistenceRecordTypeWAL, []byte("two")); !errors.Is(err, errInjectedPartialWrite) {
		t.Fatalf("append error = %v, want injected partial-write failure", err)
	}
	if len(adapter.records) != 1 || adapter.records[0].Sequence != 1 {
		t.Fatalf("records after partial write = %+v, want exactly durable sequence 1", adapter.records)
	}
}

func TestPersistenceFailureInjectionChecksumCorruptionIsRejectedAtomically(t *testing.T) {
	context := persistenceTestContext()
	digest := PersistenceContextDigest(context)
	adapter := newPersistenceTestAdapter(digest)
	if err := adapter.append(PersistenceRecordTypeWAL, []byte("one")); err != nil {
		t.Fatal(err)
	}
	adapter.fault = persistenceFaultChecksum
	if err := adapter.append(PersistenceRecordTypeWAL, []byte("two")); err != nil {
		t.Fatal(err)
	}

	target := persistenceRecoveredState{LastSequence: 77, Payloads: [][]byte{[]byte("unchanged")}}
	before := target
	if err := publishPersistenceRecovery(&target, adapter.records, digest); !errors.Is(err, ErrPersistenceChecksum) {
		t.Fatalf("recovery error = %v, want checksum error", err)
	}
	if target.LastSequence != before.LastSequence || string(target.Payloads[0]) != string(before.Payloads[0]) {
		t.Fatal("failed recovery partially published state")
	}
}

func TestPersistenceFailureInjectionContextMismatchIsRejectedAtomically(t *testing.T) {
	context := persistenceTestContext()
	digest := PersistenceContextDigest(context)
	adapter := newPersistenceTestAdapter(digest)
	if err := adapter.append(PersistenceRecordTypeWAL, []byte("one")); err != nil {
		t.Fatal(err)
	}
	adapter.fault = persistenceFaultContextMismatch
	if err := adapter.append(PersistenceRecordTypeWAL, []byte("two")); err != nil {
		t.Fatal(err)
	}

	target := persistenceRecoveredState{LastSequence: 55, Payloads: [][]byte{[]byte("unchanged")}}
	if err := publishPersistenceRecovery(&target, adapter.records, digest); !errors.Is(err, ErrPersistenceContextMismatch) {
		t.Fatalf("recovery error = %v, want context mismatch", err)
	}
	if target.LastSequence != 55 || string(target.Payloads[0]) != "unchanged" {
		t.Fatal("context mismatch partially published recovery state")
	}
}

func TestPersistenceFailureInjectionSequenceGapIsRejected(t *testing.T) {
	context := persistenceTestContext()
	digest := PersistenceContextDigest(context)
	adapter := newPersistenceTestAdapter(digest)
	if err := adapter.append(PersistenceRecordTypeWAL, []byte("one")); err != nil {
		t.Fatal(err)
	}
	adapter.fault = persistenceFaultSequenceGap
	if err := adapter.append(PersistenceRecordTypeWAL, []byte("three")); err != nil {
		t.Fatal(err)
	}

	if _, err := recoverPersistence(adapter.records, digest); !errors.Is(err, ErrPersistenceSequence) {
		t.Fatalf("recovery error = %v, want sequence error", err)
	}
}

func TestPersistenceFailureInjectionSnapshotWALReplayPublishesAtomically(t *testing.T) {
	context := persistenceTestContext()
	digest := PersistenceContextDigest(context)
	adapter := newPersistenceTestAdapter(digest)

	if err := adapter.appendAt(PersistenceRecordTypeSnapshot, 10, []byte("snapshot-10")); err != nil {
		t.Fatal(err)
	}
	if err := adapter.append(PersistenceRecordTypeWAL, []byte("wal-11")); err != nil {
		t.Fatal(err)
	}
	if err := adapter.append(PersistenceRecordTypeWAL, []byte("wal-12")); err != nil {
		t.Fatal(err)
	}

	target := persistenceRecoveredState{LastSequence: 99, Payloads: [][]byte{[]byte("old")}}
	if err := publishPersistenceRecovery(&target, adapter.records, digest); err != nil {
		t.Fatal(err)
	}
	if target.LastSequence != 12 {
		t.Fatalf("last sequence = %d, want 12", target.LastSequence)
	}
	if len(target.Payloads) != 3 || string(target.Payloads[0]) != "snapshot-10" ||
		string(target.Payloads[1]) != "wal-11" || string(target.Payloads[2]) != "wal-12" {
		t.Fatalf("recovered payloads = %#v", target.Payloads)
	}
}

func TestPersistenceFailureInjectionRejectsWALReplayAtOrBeforeSnapshot(t *testing.T) {
	context := persistenceTestContext()
	digest := PersistenceContextDigest(context)
	snapshot, err := NewPersistenceRecord(PersistenceRecordTypeSnapshot, 10, digest, []byte("snapshot-10"))
	if err != nil {
		t.Fatal(err)
	}
	stale, err := NewPersistenceRecord(PersistenceRecordTypeWAL, 10, digest, []byte("stale-10"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recoverPersistence([]PersistenceRecordEnvelope{snapshot, stale}, digest); !errors.Is(err, ErrPersistenceSequence) {
		t.Fatalf("recovery error = %v, want sequence error", err)
	}
}
