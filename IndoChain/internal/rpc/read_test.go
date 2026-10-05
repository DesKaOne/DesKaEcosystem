package rpc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/node"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestReadAPIChainHead(t *testing.T) {
	n, err := node.NewDevnet(storage.NewMemoryStore())
	if err != nil { t.Fatal(err) }
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"chain_head","params":[]}`))
	rec := httptest.NewRecorder()
	NewReadAPI(n).Handle(rec, req)
	var response Response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil { t.Fatal(err) }
	if response.Error != nil || response.Result == nil { t.Fatal("expected successful head response") }
}

func TestReadAPIBlockByHeightValidation(t *testing.T) {
	n, err := node.NewDevnet(storage.NewMemoryStore())
	if err != nil { t.Fatal(err) }
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"jsonrpc":"2.0","id":2,"method":"block_by_height","params":[]}`))
	rec := httptest.NewRecorder()
	NewReadAPI(n).Handle(rec, req)
	var response Response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil { t.Fatal(err) }
	if response.Error == nil || response.Error.Code != -32602 { t.Fatalf("unexpected error: %+v", response.Error) }
}

func TestReadAPIMethodNotFound(t *testing.T) {
	n, err := node.NewDevnet(storage.NewMemoryStore())
	if err != nil { t.Fatal(err) }
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"jsonrpc":"2.0","id":3,"method":"unknown","params":[]}`))
	rec := httptest.NewRecorder()
	NewReadAPI(n).Handle(rec, req)
	var response Response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil { t.Fatal(err) }
	if response.Error == nil || response.Error.Code != -32601 { t.Fatalf("unexpected error: %+v", response.Error) }
}
