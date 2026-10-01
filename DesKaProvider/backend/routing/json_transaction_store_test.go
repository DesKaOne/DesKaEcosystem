package routing

import (
    "context"
    "errors"
    "os"
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

func TestJSONFileTransactionStoreRejectsTerminalPaymentReferenceMutation(t *testing.T) {
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
        ProviderReference: "provider-ref-1",
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
    if err := store.PutIfCurrent(req.ReferenceID, pending, terminal); err != nil {
        t.Fatal(err)
    }

    mutated := terminal
    mutated.Payment = &payment.Transaction{
        ReferenceID:       terminal.Payment.ReferenceID,
        ProviderReference: "provider-ref-2",
        Amount:            terminal.Payment.Amount,
        Currency:          terminal.Payment.Currency,
        CustomerID:        terminal.Payment.CustomerID,
        Description:       terminal.Payment.Description,
        Status:             payment.StatusSuccess,
        Message:            "success",
    }
    mutated.Execution.Result = terminal.Execution.Result
    if err := store.PutIfCurrent(req.ReferenceID, terminal, mutated); !errors.Is(err, ErrReferenceConflict) {
        t.Fatalf("expected terminal provider-reference mutation conflict, got %v", err)
    }

    durable, ok := store.Get(req.ReferenceID)
    if !ok {
        t.Fatal("expected durable transaction to remain")
    }
    if durable.Payment.ProviderReference != "provider-ref-1" {
        t.Fatalf("provider reference ownership changed after rejected terminal mutation: %#v", durable.Payment)
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

func TestJSONFileTransactionStoreCASRejectsStaleCrossInstanceTransition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transactions", "state.json")
	storeA, err := NewJSONFileTransactionStore(path)
	if err != nil {
		t.Fatal(err)
	}
	pending := TransactionState{
		Request: PurchaseRequest{
			ProductCode: "TEST",
			CustomerNo:  "081234567890",
			ReferenceID: "ppob-cross-instance",
			Amount:      10000,
		},
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID: "ppob-cross-instance",
				ProductCode: "TEST",
				CustomerNo:  "081234567890",
				Status:      provider.StatusPending,
			},
		},
	}
	if _, created, err := storeA.CreateIfAbsentContext(context.Background(), pending); err != nil || !created {
		t.Fatalf("expected initial durable transaction, created=%v err=%v", created, err)
	}

	storeB, err := NewJSONFileTransactionStore(path)
	if err != nil {
		t.Fatal(err)
	}
	fromA, ok := storeA.Get(pending.Request.ReferenceID)
	if !ok {
		t.Fatal("expected transaction in store A")
	}
	fromB, ok := storeB.Get(pending.Request.ReferenceID)
	if !ok {
		t.Fatal("expected transaction in store B")
	}

	success := fromA
	success.Execution.Result.Status = provider.StatusSuccess
	if err := storeA.PutIfCurrent(pending.Request.ReferenceID, fromA, success); err != nil {
		t.Fatalf("store A should commit first transition: %v", err)
	}

	failed := fromB
	failed.Execution.Result.Status = provider.StatusFailed
	if err := storeB.PutIfCurrent(pending.Request.ReferenceID, fromB, failed); !errors.Is(err, ErrTransactionStateConflict) {
		t.Fatalf("expected stale cross-instance CAS conflict, got %v", err)
	}

	restarted, err := NewJSONFileTransactionStore(path)
	if err != nil {
		t.Fatal(err)
	}
	final, ok := restarted.Get(pending.Request.ReferenceID)
	if !ok {
		t.Fatal("expected durable transaction after CAS conflict")
	}
	if final.Execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("stale instance overwrote durable state: %#v", final.Execution.Result)
	}
}

func TestJSONFileTransactionStorePutFailureDoesNotChangeExistingMemoryState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transactions", "state.json")
	store, err := NewJSONFileTransactionStore(path)
	if err != nil {
		t.Fatal(err)
	}
	pending := TransactionState{
		Request: PurchaseRequest{
			ProductCode: "TEST",
			CustomerNo:  "081234567890",
			ReferenceID: "ppob-put-rollback",
			Amount:      10000,
		},
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID: "ppob-put-rollback",
				ProductCode: "TEST",
				CustomerNo:  "081234567890",
				Status:      provider.StatusPending,
			},
		},
	}
	if err := store.Put(pending); err != nil {
		t.Fatal(err)
	}
	before := store.transactions[pending.Request.ReferenceID]

	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	store.path = filepath.Join(blocked, "state.json")

	updated := before
	updated.Execution.Result.Status = provider.StatusSuccess
	if err := store.Put(updated); err == nil {
		t.Fatal("expected persistence failure")
	}
	after, ok := store.transactions[pending.Request.ReferenceID]
	if !ok || !sameTransactionState(after, before) {
		t.Fatalf("in-memory state changed after failed persistence: before=%#v after=%#v", before, after)
	}
}


func TestJSONFileTransactionStoreRejectsCorruptStateOnRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transactions", "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewJSONFileTransactionStore(path); err == nil {
		t.Fatal("expected corrupt durable state to fail closed on restart")
	}
}
