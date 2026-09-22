package ledger

import "errors"

var (
	ErrInvalidAccount = errors.New("invalid account")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

// Account is the application-level balance owned by a DesKaCash user.
// It is intentionally independent from an IndoChain address.
type Account struct {
	ID         string
	UserID     string
	Asset      string
	Balance    Money
	Version    uint64
}

const AssetDIDR = "dIDR"

func NewAccount(id, userID string) (Account, error) {
	if id == "" || userID == "" {
		return Account{}, ErrInvalidAccount
	}

	return Account{
		ID:      id,
		UserID:  userID,
		Asset:   AssetDIDR,
		Balance: Money{},
		Version: 1,
	}, nil
}

func (a *Account) Credit(amount Money) error {
	if amount.BaseUnits < 0 {
		return ErrInvalidAccount
	}

	a.Balance.BaseUnits += amount.BaseUnits
	a.Version++
	return nil
}

func (a *Account) Debit(amount Money) error {
	if amount.BaseUnits < 0 {
		return ErrInvalidAccount
	}
	if amount.BaseUnits > a.Balance.BaseUnits {
		return ErrInsufficientFunds
	}

	a.Balance.BaseUnits -= amount.BaseUnits
	a.Version++
	return nil
}
