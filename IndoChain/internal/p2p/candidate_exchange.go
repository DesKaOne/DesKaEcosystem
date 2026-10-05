package p2p

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrNilCandidateExchange = errors.New("nil candidate exchange")
	ErrCandidateHashMismatch = errors.New("candidate hash mismatch")
	ErrCandidateHeightMismatch = errors.New("candidate height mismatch")
	ErrUnexpectedSyncPeer = errors.New("unexpected sync peer")
)

type CandidateExchange struct {
	transport Transport
	maxPayload uint32
	maxRequest uint64
}

func NewCandidateExchange(transport Transport, maxPayload uint32, maxRequest uint64) (*CandidateExchange, error) {
	if transport == nil { return nil, ErrNilTransport }
	if maxPayload == 0 || maxRequest == 0 { return nil, ErrNilCandidateExchange }
	return &CandidateExchange{transport:transport, maxPayload:maxPayload, maxRequest:maxRequest}, nil
}

// PublishCandidate sends the complete development candidate over the existing
// block message type. The consensus proposal remains the authenticated hash binding.
func (e *CandidateExchange) PublishCandidate(peer PeerID, candidate block.Block) error {
	if e == nil || e.transport == nil { return ErrNilCandidateExchange }
	payload, err := EncodeBlockDevelopment(candidate, e.maxPayload)
	if err != nil { return err }
	return e.transport.Send(peer, Message{Type:MessageTypeBlock, Payload:payload})
}

// ServeOneRequest handles exactly one height-bounded block request and replies
// with the requested canonical blocks. The caller owns the event loop.
func (e *CandidateExchange) ServeOneRequest(reader SyncReader) (PeerID, error) {
	if e == nil || e.transport == nil { return "", ErrNilCandidateExchange }
	from, msg, err := e.transport.Receive()
	if err != nil { return "", err }
	if msg.Type != MessageTypeBlockRequest { return from, ErrUnexpectedSyncMessage }
	req, err := DecodeBlockRequest(msg.Payload, e.maxRequest)
	if err != nil { return from, err }
	if reader == nil { return from, ErrNilSyncReader }
	blocks := make([]block.Block, 0, req.Limit)
	for i := uint64(0); i < req.Limit; i++ {
		b, _, err := reader.BlockByHeight(req.FromHeight + types.Height(i))
		if err != nil {
			if len(blocks) == 0 { return from, fmt.Errorf("%w: %v", ErrSyncReadFailure, err) }
			break
		}
		blocks = append(blocks, b)
	}
	payload, err := EncodeBlockResponseDevelopment(BlockResponse{Blocks:blocks}, e.maxPayload)
	if err != nil { return from, err }
	if err := e.transport.Send(from, Message{Type:MessageTypeBlockResponse, Payload:payload}); err != nil { return from, err }
	return from, nil
}

// FetchCandidate requests one block by proposal height and verifies its hash
// against the authenticated proposal payload before returning it.
func (e *CandidateExchange) FetchCandidate(peer PeerID, proposal consensus.Message) (block.Block, error) {
	if e == nil || e.transport == nil { return block.Block{}, ErrNilCandidateExchange }
	if proposal.Type != consensus.MessageTypeProposal { return block.Block{}, ErrUnexpectedSyncMessage }
	if len(proposal.Payload) != 32 { return block.Block{}, ErrCandidateHashMismatch }
	req, err := EncodeBlockRequest(BlockRequest{FromHeight:proposal.Height, Limit:1}, e.maxRequest)
	if err != nil { return block.Block{}, err }
	if err := e.transport.Send(peer, Message{Type:MessageTypeBlockRequest, Payload:req}); err != nil { return block.Block{}, err }
	from, msg, err := e.transport.Receive()
	if err != nil { return block.Block{}, err }
	if from != peer { return block.Block{}, ErrUnexpectedSyncPeer }
	if msg.Type != MessageTypeBlockResponse { return block.Block{}, ErrUnexpectedSyncMessage }
	resp, err := DecodeBlockResponseDevelopment(msg.Payload, e.maxPayload)
	if err != nil { return block.Block{}, err }
	if len(resp.Blocks) != 1 { return block.Block{}, ErrCandidateHeightMismatch }
	candidate := resp.Blocks[0]
	if candidate.Header.Height != proposal.Height { return block.Block{}, ErrCandidateHeightMismatch }
	hash, err := block.Hash(candidate)
	if err != nil { return block.Block{}, err }
	var expected types.Hash
	copy(expected[:], proposal.Payload)
	if hash != expected { return block.Block{}, ErrCandidateHashMismatch }
	return candidate, nil
}
