package ledger

// BaseUnitPerDIDR defines the smallest accounting unit.
// 1 dIDR = 1,000 base units, therefore 0.001 dIDR = 1 base unit.
const BaseUnitPerDIDR int64 = 1000

// Money stores dIDR amounts as integer base units to avoid floating-point errors.
type Money struct {
	BaseUnits int64
}

func FromDIDR(didR int64) Money {
	return Money{BaseUnits: didR * BaseUnitPerDIDR}
}

func (m Money) DIDR() int64 {
	return m.BaseUnits / BaseUnitPerDIDR
}

func (m Money) IsZero() bool {
	return m.BaseUnits == 0
}
