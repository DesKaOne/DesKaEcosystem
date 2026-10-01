package routing

import (
    "context"
    "errors"
    "path/filepath"
    "testing"

    provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
    payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)

func TestJSONFileTransactionStorePersistsPaymentIdentityAcrossRestart(t *testing.T) {
    path := filepath.Join(t.TempDir(), "transactions", "state.json")
    store, err := NewJSONFileTransactionStore(path)
    if err != nil {
        t.Fatal(err)
    }

    req := payment.PaymentRequest{
        ReferenceID: "pay-json-restart",
        Amount:      50000,
        Currency:    "IDR",
        CustomerID:  "cust-1",
        Description: "test payment",
    }
    pending, err := NewPaymentTransactionState(req, "mock")
    if err != nil {
        t.Fatal(err)
    }

    claimed, created, err := store.CreateIfAbsentContext(context.Background(), pending)
    if err != nil {
        t.Fatal(err)
    }
    if !created || !sameTransactionIdentity(claimed, pending) {
        t.Fatalf("expected payment transaction to be atomically created, created=%v state=%#v", created, claimed)
    }

    restarted, err := NewJSONFileTransactionStore(path)
    if err != nil {
        t.Fatal(err)
    }
    recovered, ok := restarted.Get(req.ReferenceID)
    if !ok {
        t.Fatal("expected payment transaction after restart")
    }
    if !sameTransactionIdentity(recovered, pending) {
        t.Fatalf("recovered payment identity changed: %#v", recovered)
    }
}

func TestJSONFileTransactionStoreRejectsPaymentReferenceOwnershipChange(t *testing.T) {
    store, err := NewJSONFileTransactionStore(filepath.Join(t.TempDir(), "transactions", "state.json"))
    if err != nil {
        t.Fatal(err)
    }

    req := payment.PaymentRequest{
        ReferenceID: "pay-provider-reference",
        Amount:      50000,
        Currency:    "IDR",
        CustomerID:  "cust-1",
        Description: "test payment",
    }
    pending, err := NewPaymentTransactionState(req, "mock")
    if err != nil {
        t.Fatal(err)
    }
    pending.Payment.ProviderReference = "provider-ref-1"
    if err := store.Put(pending); err != nil {
        t.Fatal(err)
    }

    terminal := pending
    terminal.Payment = &payment.Transaction{
        ReferenceID:       pending.Payment.ReferenceID,
        ProviderReference: "provider-ref-2",
        Amount:            pending.Payment.Amount,
        Currency:          pending.Payment.Currency,
        CustomerID:        pending.Payment.CustomerID,
        Description:       pending.Payment.Description,
        Status:             payment.StatusSuccess,
        Message:            "success",
    }
    terminal.Execution.Result = provider.PurchaseResult{
        ReferenceID: req.ReferenceID,
        Status: provider.TransactionStatus(payment.StatusSuccess),
    }

    if err := store.PutIfCurrent(req.ReferenceID, pending, terminal); !errors.Is(err, ErrReferenceConflict) {
        t.Fatalf("expected provider-reference ownership conflict, got %v", err)
    }

    durable, ok := store.Get(req.ReferenceID)
    if !ok {
        t.Fatal("expected durable transaction to remain")
    }
    if durable.Payment.ProviderReference != "provider-ref-1" {
        t.Fatalf("provider reference ownership changed after rejected transition: %#v", durable.Payment)
    }
}

func TestJSONFileTransactionStoreAcceptsInitialProviderReferenceOnTransition(t *testing.T) {
    store, err := NewJSONFileTransactionStore(filepath.Join(t.TempDir(), "transactions", "state.json"))
    if err != nil {
        t.Fatal(err)
    }

    req := payment.PaymentRequest{
        ReferenceID: "pay-provider-reference-initial",
        Amount:      50000,
        Currency:    "IDR",
        CustomerID:  "cust-1",
        Description: "test payment",
    }
    pending, err := NewPaymentTransactionState(req, "mock")
    if err != nil {
        t.Fatal(err)
    }
    if err := store.Put(pending); err != nil {
        t.Fatal(err)
    }

    terminal := pending
    terminal.Payment = &payment.Transaction{
        ReferenceID:       pending.Payment.ReferenceID,
        ProviderReference: "provider-ref-1",
        Amount:            pending.Payment.Amount,
        Currency:          pending.Payment.Currency,
        CustomerID:        pending.Payment.CustomerID,
        Description:        pending.Payment.Description,
        Status:             payment.StatusSuccess,
        Message:            "success",
    }
    terminal.Execution.Result = provider.PurchaseResult{
        ReferenceID: req.ReferenceID,
        Status: provider.TransactionStatus(payment.StatusSuccess),
    }
    if err := store.PutIfCurrent(req.ReferenceID, pending, terminal); err != nil {
        t.Fatalf("expected initial provider reference to be accepted: %v", err)
    }
}
