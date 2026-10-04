package consensus

import (
	"errors"
	"testing"
)

func TestValidateMessageSenderAcceptsActiveValidator(t *testing.T) {
	set, err := NewValidatorSet([][]byte{{2}, {1}})
	if err != nil {
		t.Fatal(err)
	}
	msg := Message{Sender: []byte{1}}
	if err := ValidateMessageSender(msg, set); err != nil {
		t.Fatalf("expected active validator, got %v", err)
	}
}

func TestValidateMessageSenderRejectsUnknownValidator(t *testing.T) {
	set, _ := NewValidatorSet([][]byte{{1}})
	msg := Message{Sender: []byte{2}}
	if err := ValidateMessageSender(msg, set); !errors.Is(err, ErrConsensusMessageUnauthorized) {
		t.Fatalf("expected unauthorized sender, got %v", err)
	}
}

func TestValidateMessageSenderRejectsMissingSender(t *testing.T) {
	set, _ := NewValidatorSet([][]byte{{1}})
	if err := ValidateMessageSender(Message{}, set); !errors.Is(err, ErrMissingSender) {
		t.Fatalf("expected missing sender, got %v", err)
	}
}

func TestValidateMessageSenderRejectsInvalidSet(t *testing.T) {
	set := ValidatorSet{Validators: [][]byte{{2}, {1}}}
	if err := ValidateMessageSender(Message{Sender: []byte{1}}, set); !errors.Is(err, ErrInvalidValidatorSet) {
		t.Fatalf("expected invalid validator set, got %v", err)
	}
}
