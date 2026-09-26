package p2p

import (
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"errors"
	"testing"
)

type testSyncResponseEncoder struct {
	payload []byte
	err     error
}

func (e testSyncResponseEncoder) EncodeBlockResponse(BlockResponse) ([]byte, error) {
	if e.err != nil {
		return nil, e.err
	}
	return append([]byte(nil), e.payload...), nil
}

func TestBuildBlockResponseMessage(t *testing.T) {
	msg, err := BuildBlockResponseMessage(
		BlockResponse{Blocks: []block.Block{{}}},
		testSyncResponseEncoder{payload: []byte{1, 2, 3}},
		16,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != MessageTypeBlockResponse {
		t.Fatalf("unexpected type: %v", msg.Type)
	}
	if string(msg.Payload) != string([]byte{1, 2, 3}) {
		t.Fatalf("unexpected payload: %v", msg.Payload)
	}
}

func TestBuildBlockResponseMessageRejectsNilEncoder(t *testing.T) {
	_, err := BuildBlockResponseMessage(BlockResponse{}, nil, 16)
	if !errors.Is(err, ErrNilSyncResponseEncoder) {
		t.Fatalf("expected nil encoder error, got %v", err)
	}
}

func TestBuildBlockResponseMessagePropagatesEncoderError(t *testing.T) {
	want := errors.New("encode failed")
	_, err := BuildBlockResponseMessage(
		BlockResponse{},
		testSyncResponseEncoder{err: want},
		16,
	)
	if !errors.Is(err, want) {
		t.Fatalf("expected encoder error, got %v", err)
	}
}

func TestBuildBlockResponseMessageRejectsPayloadLimit(t *testing.T) {
	_, err := BuildBlockResponseMessage(
		BlockResponse{},
		testSyncResponseEncoder{payload: []byte{1, 2, 3}},
		2,
	)
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("expected invalid message error, got %v", err)
	}
}
