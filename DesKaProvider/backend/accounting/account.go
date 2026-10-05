package accounting

import (
	"errors"
	"fmt"
)

type AccountType string

const (
	AccountTypeMain       AccountType = "MAIN"
	AccountTypeSettlementIn  AccountType = "SETTLEMENT_IN"
	AccountTypeSettlementOut AccountType = "SETTLEMENT_OUT"
	AccountTypeReserve     AccountType = "RESERVE"
	AccountTypeFee         AccountType = "FEE"
	AccountTypeEscrow      AccountType = "ESCROW"
	AccountTypeClearing    AccountType = "CLEARING"
)

type Account struct {
	ID       string
	Type     AccountType
	OwnerID  string
	Currency string
	Name     string
	Active   bool
}

func (a Account) Validate() error {
	if a.ID == "" || a.Currency == "" || a.Name == "" {
		return errors.New("account id, currency, and name are required")
	}
	if !validAccountType(a.Type) {
		return fmt.Errorf("unsupported account type %q", a.Type)
	}
	return nil
}

func validAccountType(t AccountType) bool {
	switch t {
	case AccountTypeMain, AccountTypeSettlementIn, AccountTypeSettlementOut,
		AccountTypeReserve, AccountTypeFee, AccountTypeEscrow, AccountTypeClearing:
		return true
	default:
		return false
	}
}
