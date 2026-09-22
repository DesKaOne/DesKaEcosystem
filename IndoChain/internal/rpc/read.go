package rpc

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/node"
)

var (
	ErrInvalidRequest = errors.New("invalid rpc request")
	ErrMethodNotFound = errors.New("rpc method not found")
)

type Request struct {
	JSONRPC string
	ID uint64
	Method string
	Params json.RawMessage
}

type Response struct {
	JSONRPC string
	ID uint64
	Result any
	Error *Error
}

type Error struct {
	Code int
	Message string
}

type ReadAPI struct { Reader node.ChainReader }

func NewReadAPI(reader node.ChainReader) *ReadAPI { return &ReadAPI{Reader: reader} }

func (api *ReadAPI) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, req.ID, -32600, ErrInvalidRequest.Error())
		return
	}
	if req.JSONRPC != "2.0" || req.Method == "" || api == nil || api.Reader == nil {
		writeError(w, req.ID, -32600, ErrInvalidRequest.Error())
		return
	}
	result, rpcErr := api.call(req.Method, req.Params)
	if rpcErr != nil {
		writeError(w, req.ID, rpcErr.Code, rpcErr.Message)
		return
	}
	_ = json.NewEncoder(w).Encode(Response{JSONRPC:"2.0", ID:req.ID, Result:result})
}

func (api *ReadAPI) call(method string, params json.RawMessage) (any, *Error) {
	switch method {
	case "chain_head":
		b, hash, err := api.Reader.HeadBlock()
		if err != nil { return nil, mapError(err) }
		return map[string]any{"block":b, "hash":hash.String()}, nil
	case "block_by_height":
		var p []uint64
		if err := json.Unmarshal(params, &p); err != nil || len(p) != 1 {
			return nil, &Error{Code:-32602, Message:"expected one height parameter"}
		}
		b, hash, err := api.Reader.BlockByHeight(types.Height(p[0]))
		if err != nil { return nil, mapError(err) }
		return map[string]any{"block":b, "hash":hash.String()}, nil
	case "transaction_by_hash":
		var p []string
		if err := json.Unmarshal(params, &p); err != nil || len(p) != 1 {
			return nil, &Error{Code:-32602, Message:"expected one hash parameter"}
		}
		raw, err := hex.DecodeString(p[0])
		if err != nil || len(raw) != 32 {
			return nil, &Error{Code:-32602, Message:"expected 32-byte hex transaction hash"}
		}
		var hash types.Hash
		copy(hash[:], raw)
		record, err := api.Reader.TransactionByHash(hash)
		if err != nil { return nil, mapError(err) }
		return record, nil
	default:
		return nil, &Error{Code:-32601, Message:ErrMethodNotFound.Error()}
	}
}

func mapError(err error) *Error {
	if errors.Is(err, node.ErrTransactionNotFound) {
		return &Error{Code:-32004, Message:err.Error()}
	}
	if errors.Is(err, node.ErrNilNode) {
		return &Error{Code:-32000, Message:err.Error()}
	}
	return &Error{Code:-32000, Message:err.Error()}
}

func writeError(w http.ResponseWriter, id uint64, code int, message string) {
	_ = json.NewEncoder(w).Encode(Response{JSONRPC:"2.0", ID:id, Error:&Error{Code:code, Message:message}})
}
