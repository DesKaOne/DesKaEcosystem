package p2p

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidBlockPayload = errors.New("invalid block payload")
	ErrBlockPayloadTooLarge = errors.New("block payload too large")
)

const blockWireVersion byte = 1

// EncodeBlockDevelopment serializes the existing Block model for development
// P2P candidate exchange. It is not the canonical block serialization.
func EncodeBlockDevelopment(b block.Block, maxBytes uint32) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(blockWireVersion)
	putU16Block(&buf, uint16(b.Header.Version))
	putU64Block(&buf, uint64(b.Header.ChainID))
	putU64Block(&buf, uint64(b.Header.Height))
	putU64Block(&buf, uint64(b.Header.Timestamp))
	buf.Write(b.Header.PreviousHash[:])
	buf.Write(b.Header.TransactionsRoot[:])
	buf.Write(b.Header.StateRoot[:])
	if err := putFieldBlock(&buf, b.Header.Proposer); err != nil { return nil, err }
	if err := putFieldBlock(&buf, b.Header.ConsensusEvidence); err != nil { return nil, err }
	if uint64(len(b.Transactions)) > uint64(^uint32(0)) { return nil, ErrBlockPayloadTooLarge }
	putU32Block(&buf, uint32(len(b.Transactions)))
	for i, raw := range b.Transactions {
		tx, ok := raw.(transaction.Transaction)
		if !ok { return nil, fmt.Errorf("%w: transaction %d", ErrInvalidBlockPayload, i) }
		encoded, err := transaction.SignedBytes(tx)
		if err != nil { return nil, fmt.Errorf("transaction %d: %w", i, err) }
		if err := putFieldBlock(&buf, encoded); err != nil { return nil, err }
	}
	if maxBytes > 0 && uint32(buf.Len()) > maxBytes { return nil, ErrBlockPayloadTooLarge }
	return buf.Bytes(), nil
}

// DecodeBlockDevelopment decodes the development candidate exchange format.
func DecodeBlockDevelopment(data []byte, maxBytes uint32) (block.Block, error) {
	if len(data) == 0 || (maxBytes > 0 && uint32(len(data)) > maxBytes) { return block.Block{}, ErrBlockPayloadTooLarge }
	r := bytes.NewReader(data)
	version, err := r.ReadByte()
	if err != nil || version != blockWireVersion { return block.Block{}, ErrInvalidBlockPayload }
	pv, err := readU16Block(r); if err != nil { return block.Block{}, err }
	chainID, err := readU64Block(r); if err != nil { return block.Block{}, err }
	height, err := readU64Block(r); if err != nil { return block.Block{}, err }
	ts, err := readU64Block(r); if err != nil { return block.Block{}, err }
	prev, err := readHashBlock(r); if err != nil { return block.Block{}, err }
	txRoot, err := readHashBlock(r); if err != nil { return block.Block{}, err }
	stateRoot, err := readHashBlock(r); if err != nil { return block.Block{}, err }
	proposer, err := readFieldBlock(r); if err != nil { return block.Block{}, err }
	evidence, err := readFieldBlock(r); if err != nil { return block.Block{}, err }
	count, err := readU32Block(r); if err != nil { return block.Block{}, err }
	if uint64(count) > uint64(r.Len()) { return block.Block{}, ErrInvalidBlockPayload }
	txs := make([]any, 0, count)
	for i := uint32(0); i < count; i++ {
		encoded, err := readFieldBlock(r); if err != nil { return block.Block{}, err }
		tx, err := decodeSignedTransaction(encoded)
		if err != nil { return block.Block{}, fmt.Errorf("transaction %d: %w", i, err) }
		txs = append(txs, tx)
	}
	if r.Len() != 0 { return block.Block{}, ErrInvalidBlockPayload }
	return block.Block{Header:block.Header{
		Version: types.ProtocolVersion(pv), ChainID: types.ChainID(chainID), Height: types.Height(height),
		Timestamp: int64(ts), PreviousHash: prev, TransactionsRoot: txRoot, StateRoot: stateRoot,
		Proposer: proposer, ConsensusEvidence: evidence,
	}, Transactions: txs}, nil
}

func EncodeBlockResponseDevelopment(resp BlockResponse, maxBytes uint32) ([]byte, error) {
	var buf bytes.Buffer
	if uint64(len(resp.Blocks)) > uint64(^uint32(0)) { return nil, ErrBlockPayloadTooLarge }
	putU32Block(&buf, uint32(len(resp.Blocks)))
	for i, b := range resp.Blocks {
		encoded, err := EncodeBlockDevelopment(b, maxBytes)
		if err != nil { return nil, fmt.Errorf("block %d: %w", i, err) }
		if err := putFieldBlock(&buf, encoded); err != nil { return nil, err }
	}
	if maxBytes > 0 && uint32(buf.Len()) > maxBytes { return nil, ErrBlockPayloadTooLarge }
	return buf.Bytes(), nil
}

func DecodeBlockResponseDevelopment(data []byte, maxBytes uint32) (BlockResponse, error) {
	if len(data) == 0 || (maxBytes > 0 && uint32(len(data)) > maxBytes) { return BlockResponse{}, ErrBlockPayloadTooLarge }
	r := bytes.NewReader(data)
	count, err := readU32Block(r); if err != nil { return BlockResponse{}, err }
	blocks := make([]block.Block, 0, count)
	for i := uint32(0); i < count; i++ {
		encoded, err := readFieldBlock(r); if err != nil { return BlockResponse{}, err }
		b, err := DecodeBlockDevelopment(encoded, maxBytes); if err != nil { return BlockResponse{}, fmt.Errorf("block %d: %w", i, err) }
		blocks = append(blocks, b)
	}
	if r.Len() != 0 { return BlockResponse{}, ErrInvalidBlockPayload }
	return BlockResponse{Blocks: blocks}, nil
}

func decodeSignedTransaction(data []byte) (transaction.Transaction, error) {
	r := bytes.NewReader(data)
	pv, err := readU16Block(r); if err != nil { return transaction.Transaction{}, err }
	chainID, err := readU64Block(r); if err != nil { return transaction.Transaction{}, err }
	nonce, err := readU64Block(r); if err != nil { return transaction.Transaction{}, err }
	sender, err := readFieldBlock(r); if err != nil { return transaction.Transaction{}, err }
	recipient, err := readFieldBlock(r); if err != nil { return transaction.Transaction{}, err }
	value, err := readU64Block(r); if err != nil { return transaction.Transaction{}, err }
	gas, err := readU64Block(r); if err != nil { return transaction.Transaction{}, err }
	dataField, err := readFieldBlock(r); if err != nil { return transaction.Transaction{}, err }
	sig, err := readFieldBlock(r); if err != nil { return transaction.Transaction{}, err }
	if r.Len() != 0 { return transaction.Transaction{}, ErrInvalidBlockPayload }
	return transaction.Transaction{Version:types.ProtocolVersion(pv), ChainID:types.ChainID(chainID), Nonce:types.Nonce(nonce), Sender:sender, Recipient:recipient, Value:value, GasLimit:gas, Data:dataField, Signature:sig}, nil
}

func putU16Block(b *bytes.Buffer, v uint16) { var x [2]byte; binary.BigEndian.PutUint16(x[:], v); b.Write(x[:]) }
func putU32Block(b *bytes.Buffer, v uint32) { var x [4]byte; binary.BigEndian.PutUint32(x[:], v); b.Write(x[:]) }
func putU64Block(b *bytes.Buffer, v uint64) { var x [8]byte; binary.BigEndian.PutUint64(x[:], v); b.Write(x[:]) }
func putFieldBlock(b *bytes.Buffer, v []byte) error { if uint64(len(v)) > uint64(^uint32(0)) { return ErrBlockPayloadTooLarge }; putU32Block(b, uint32(len(v))); b.Write(v); return nil }

func readU16Block(r *bytes.Reader) (uint16,error) { var x [2]byte; if _,e:=r.Read(x[:]);e!=nil{return 0,ErrInvalidBlockPayload}; return binary.BigEndian.Uint16(x[:]),nil }
func readU32Block(r *bytes.Reader) (uint32,error) { var x [4]byte; if _,e:=r.Read(x[:]);e!=nil{return 0,ErrInvalidBlockPayload}; return binary.BigEndian.Uint32(x[:]),nil }
func readU64Block(r *bytes.Reader) (uint64,error) { var x [8]byte; if _,e:=r.Read(x[:]);e!=nil{return 0,ErrInvalidBlockPayload}; return binary.BigEndian.Uint64(x[:]),nil }
func readHashBlock(r *bytes.Reader) (types.Hash,error) { var h types.Hash; if _,e:=r.Read(h[:]);e!=nil{return h,ErrInvalidBlockPayload}; return h,nil }
func readFieldBlock(r *bytes.Reader) ([]byte,error) { n,e:=readU32Block(r); if e!=nil{return nil,e}; if uint64(n)>uint64(r.Len()){return nil,ErrInvalidBlockPayload}; out:=make([]byte,n); if _,e:=r.Read(out);e!=nil{return nil,ErrInvalidBlockPayload}; return out,nil }
