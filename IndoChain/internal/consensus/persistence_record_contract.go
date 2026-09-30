package consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	PersistenceRecordFormatVersion uint16 = 1
	PersistenceRecordTypeSnapshot  uint8  = 1
	PersistenceRecordTypeWAL       uint8  = 2
)

var (
	ErrInvalidPersistenceRecord = errors.New("invalid consensus persistence record")
	ErrPersistenceContextMismatch = errors.New("consensus persistence context mismatch")
	ErrPersistenceSequence = errors.New("invalid consensus persistence sequence")
	ErrPersistenceChecksum = errors.New("consensus persistence checksum mismatch")
	ErrPersistenceRecordVersion = errors.New("unsupported consensus persistence record version")
)

type PersistenceRecordEnvelope struct {
	FormatVersion uint16
	RecordType    uint8
	Sequence      uint64
	ContextDigest [32]byte
	Payload       []byte
	Checksum      [32]byte
}

type PersistenceSnapshotBoundary struct {
	FormatVersion uint16
	BaseSequence  uint64
	ContextDigest [32]byte
	Payload       []byte
	Checksum      [32]byte
}

func NewPersistenceRecord(recordType uint8, sequence uint64, contextDigest [32]byte, payload []byte) (PersistenceRecordEnvelope, error) {
	if recordType != PersistenceRecordTypeSnapshot && recordType != PersistenceRecordTypeWAL {
		return PersistenceRecordEnvelope{}, fmt.Errorf("%w: record type %d", ErrInvalidPersistenceRecord, recordType)
	}
	if sequence == 0 {
		return PersistenceRecordEnvelope{}, ErrPersistenceSequence
	}
	record := PersistenceRecordEnvelope{
		FormatVersion: PersistenceRecordFormatVersion,
		RecordType:    recordType,
		Sequence:      sequence,
		ContextDigest: contextDigest,
		Payload:       append([]byte(nil), payload...),
	}
	record.Checksum = record.computeChecksum()
	return record, nil
}

func (r PersistenceRecordEnvelope) Validate(expectedContext [32]byte, previousSequence uint64) error {
	if r.FormatVersion != PersistenceRecordFormatVersion {
		return ErrPersistenceRecordVersion
	}
	if r.RecordType != PersistenceRecordTypeSnapshot && r.RecordType != PersistenceRecordTypeWAL {
		return ErrInvalidPersistenceRecord
	}
	if r.Sequence == 0 {
		return ErrPersistenceSequence
	}
	if previousSequence > 0 && r.Sequence != previousSequence+1 {
		return ErrPersistenceSequence
	}
	if !bytes.Equal(r.ContextDigest[:], expectedContext[:]) {
		return ErrPersistenceContextMismatch
	}
	expectedChecksum := r.computeChecksum()
	if !bytes.Equal(r.Checksum[:], expectedChecksum[:]) {
		return ErrPersistenceChecksum
	}
	return nil
}

func (r PersistenceRecordEnvelope) computeChecksum() [32]byte {
	h := sha256.New()
	var version [2]byte
	binary.BigEndian.PutUint16(version[:], r.FormatVersion)
	h.Write(version[:])
	h.Write([]byte{r.RecordType})
	var sequence [8]byte
	binary.BigEndian.PutUint64(sequence[:], r.Sequence)
	h.Write(sequence[:])
	h.Write(r.ContextDigest[:])
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(r.Payload)))
	h.Write(length[:])
	h.Write(r.Payload)
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func PersistenceContextDigest(context PersistenceContext) [32]byte {
	sum := sha256.Sum256(context.CanonicalBytes())
	return sum
}

type PersistenceContext struct {
	ProtocolVersion uint64
	ChainID         uint64
	Epoch           uint64
	Height          uint64
	Round           uint64
	Phase           uint8
	ValidatorDigest [32]byte
	VotingPowerDigest [32]byte
	ThresholdNumerator uint64
	ThresholdDenominator uint64
	ProposerPolicy string
	ProposerPolicyVersion string
}

func (c PersistenceContext) CanonicalBytes() []byte {
	buf := bytes.NewBuffer(nil)
	var u64 [8]byte
	put := func(v uint64) {
		binary.BigEndian.PutUint64(u64[:], v)
		buf.Write(u64[:])
	}
	put(c.ProtocolVersion)
	put(c.ChainID)
	put(c.Epoch)
	put(c.Height)
	put(c.Round)
	buf.WriteByte(c.Phase)
	buf.Write(c.ValidatorDigest[:])
	buf.Write(c.VotingPowerDigest[:])
	put(c.ThresholdNumerator)
	put(c.ThresholdDenominator)
	putString := func(s string) {
		put(uint64(len(s)))
		buf.WriteString(s)
	}
	putString(c.ProposerPolicy)
	putString(c.ProposerPolicyVersion)
	return buf.Bytes()
}

func PersistenceChecksumHex(sum [32]byte) string {
	return hex.EncodeToString(sum[:])
}
