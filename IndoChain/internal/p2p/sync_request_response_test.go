package p2p

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type testBlockResponseEncoder struct {
	payload []byte
	err     error
}

func (e testBlockResponseEncoder) EncodeBlockResponse(resp BlockResponse) ([]byte, error) {
	if e.err != nil {
		return nil, e.err
	}
	return e.payload, nil
}

func TestSyncRequestServiceHandleMessage(t *testing.T) {
	service := &SyncRequestService{
		Handler: &SyncMessageHandler{
			Service: &SyncService{
				Reader: rangeReader{blocks: map[types.Height]block.Block{
					1: {Header: block.Header{Height: 1}},
				}},
				MaxLimit: 2,
			},
			MaxPayload: 16,
			MaxLimit:   2,
		},
	}
	payload, err := EncodeBlockRequest(BlockRequest{FromHeight: 1, Limit: 1}, 2)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	msg, err := service.HandleMessage(
		Message{Type: MessageTypeBlockRequest, Payload: payload},
		testBlockResponseEncoder{payload: []byte{9, 8, 7}},
		16,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != MessageTypeBlockResponse {
		t.Fatalf("got message type %v, want block response", msg.Type)
	}
	if len(msg.Payload) != 3 {
		t.Fatalf("got payload length %d, want 3", len(msg.Payload))
	}
}

func TestSyncRequestServiceHandleMessageRejectsNilEncoder(t *testing.T) {
	service := &SyncRequestService{Handler: &SyncMessageHandler{
		Service: &SyncService{Reader: rangeReader{blocks: map[types.Height]block.Block{1: {Header: block.Header{Height: 1}}}}, MaxLimit: 2},
		MaxPayload: 16,
		MaxLimit: 2,
	}}
	payload, err := EncodeBlockRequest(BlockRequest{FromHeight: 1, Limit: 1}, 2)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	_, err = service.HandleMessage(Message{Type: MessageTypeBlockRequest, Payload: payload}, nil, 16)
	if !errors.Is(err, ErrNilSyncRequestResponseEncoder) {
		t.Fatalf("expected nil encoder error, got %v", err)
	}
}

func TestSyncRequestServiceHandleMessagePreservesRequestError(t *testing.T) {
	service := &SyncRequestService{Handler: &SyncMessageHandler{
		Service: &SyncService{Reader: rangeReader{}, MaxLimit: 2},
		MaxPayload: 16,
		MaxLimit: 2,
	}}
	_, err := service.HandleMessage(Message{Type: MessageTypeBlockRequest, Payload: make([]byte, 16)}, testBlockResponseEncoder{payload: []byte{1}}, 16)
	if !errors.Is(err, ErrSyncReadFailure) {
		t.Fatalf("expected sync read failure, got %v", err)
	}
}

func TestSyncRequestServiceHandleMessagePreservesEncoderError(t *testing.T) {
	service := &SyncRequestService{Handler: &SyncMessageHandler{
		Service: &SyncService{Reader: rangeReader{blocks: map[types.Height]block.Block{1: {Header: block.Header{Height: 1}}}}, MaxLimit: 2},
		MaxPayload: 16,
		MaxLimit: 2,
	}}
	payload, err := EncodeBlockRequest(BlockRequest{FromHeight: 1, Limit: 1}, 2)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	_, err = service.HandleMessage(Message{Type: MessageTypeBlockRequest, Payload: payload}, testBlockResponseEncoder{err: errors.New("encode failed")}, 16)
	if err == nil || err.Error() != "encode failed" {
		t.Fatalf("expected encoder error, got %v", err)
	}
}
