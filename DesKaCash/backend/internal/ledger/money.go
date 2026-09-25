package ledger

// Money stores integer base units. The meaning of a base unit is determined by
// the Asset carried alongside the Money value.
//
// IDR uses rupiah as its base unit: 1 IDR = 1 base unit.
// dIDR uses 0.001 dIDR as its base unit: 1 dIDR = 1,000 base units.
type Money struct {
	BaseUnits int64
}

const BaseUnitPerDIDR int64 = 1000

func FromIDR(idr int64) Money {
	return Money{BaseUnits: idr}
}

func FromDIDR(didR int64) Money {
	return Money{BaseUnits: didR * BaseUnitPerDIDR}
}

func (m Money) IDR() int64 {
	return m.BaseUnits
}

func (m Money) DIDR() int64 {
	return m.BaseUnits / BaseUnitPerDIDR
}

func (m Money) IsZero() bool {
	return m.BaseUnits == 0
}
