package consensus

import (
    "bytes"
    "encoding/binary"
    "errors"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var ErrConsensusMessageEncoding = errors.New("invalid consensus message encoding")

const consensusMessageWireVersion byte = 1

// EncodeMessage serializes a consensus message into a deterministic wire payload.
func EncodeMessage(m Message, rules ValidationRules) ([]byte, error) {
    if err := ValidateMessage(m, rules); err != nil {
        return nil, err
    }
    var b bytes.Buffer
    b.WriteByte(consensusMessageWireVersion)
    var x [8]byte
    binary.BigEndian.PutUint16(x[:2], uint16(m.ProtocolVersion)); b.Write(x[:2])
    binary.BigEndian.PutUint64(x[:], uint64(m.ChainID)); b.Write(x[:])
    binary.BigEndian.PutUint64(x[:], m.Epoch); b.Write(x[:])
    binary.BigEndian.PutUint64(x[:], uint64(m.Height)); b.Write(x[:])
    binary.BigEndian.PutUint64(x[:], m.Round); b.Write(x[:])
    putU32(&b, uint32(len(m.Sender))); b.Write(m.Sender)
    b.WriteByte(byte(m.Type))
    putU32(&b, uint32(len(m.Payload))); b.Write(m.Payload)
    putU32(&b, uint32(len(m.Signature))); b.Write(m.Signature)
    return b.Bytes(), nil
}

// DecodeMessage deserializes and validates a consensus wire payload.
func DecodeMessage(data []byte, rules ValidationRules) (Message, error) {
    if len(data) < 1+2+8+8+8+8+4+1+4+4 {
        return Message{}, ErrConsensusMessageEncoding
    }
    r := bytes.NewReader(data)
    version, err := r.ReadByte()
    if err != nil || version != consensusMessageWireVersion { return Message{}, ErrConsensusMessageEncoding }
    var u16 [2]byte
    if _, err := r.Read(u16[:]); err != nil { return Message{}, ErrConsensusMessageEncoding }
    var u64 [8]byte
    readU64 := func() (uint64, error) {
        if _, err := r.Read(u64[:]); err != nil { return 0, ErrConsensusMessageEncoding }
        return binary.BigEndian.Uint64(u64[:]), nil
    }
    chainID, err := readU64(); if err != nil { return Message{}, err }
    epoch, err := readU64(); if err != nil { return Message{}, err }
    height, err := readU64(); if err != nil { return Message{}, err }
    round, err := readU64(); if err != nil { return Message{}, err }
    sender, err := readField(r); if err != nil { return Message{}, err }
    typ, err := r.ReadByte(); if err != nil { return Message{}, ErrConsensusMessageEncoding }
    payload, err := readField(r); if err != nil { return Message{}, err }
    signature, err := readField(r); if err != nil { return Message{}, err }
    if r.Len() != 0 { return Message{}, ErrConsensusMessageEncoding }
    m := Message{
        ProtocolVersion: types.ProtocolVersion(binary.BigEndian.Uint16(u16[:])),
        ChainID: types.ChainID(chainID),
        Epoch: epoch,
        Height: types.Height(height),
        Round: round,
        Sender: sender,
        Type: MessageType(typ),
        Payload: payload,
        Signature: signature,
    }
    if err := ValidateMessage(m, rules); err != nil { return Message{}, err }
    return m, nil
}

func putU32(b *bytes.Buffer, v uint32) {
    var x [4]byte
    binary.BigEndian.PutUint32(x[:], v)
    b.Write(x[:])
}

func readField(r *bytes.Reader) ([]byte, error) {
    var x [4]byte
    if _, err := r.Read(x[:]); err != nil { return nil, ErrConsensusMessageEncoding }
    n := binary.BigEndian.Uint32(x[:])
    if uint64(n) > uint64(r.Len()) { return nil, ErrConsensusMessageEncoding }
    out := make([]byte, n)
    if _, err := r.Read(out); err != nil { return nil, ErrConsensusMessageEncoding }
    return out, nil
}
