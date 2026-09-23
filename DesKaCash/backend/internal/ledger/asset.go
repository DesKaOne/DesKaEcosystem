package ledger

import "errors"

// Asset identifies the denomination carried by an application ledger balance.
// IDR is the fiat wallet asset used by DesKaCash v0.1.
// dIDR is the native on-chain asset of IndoChain and remains a separate asset.
type Asset string

const (
	AssetIDR  = "IDR"
	AssetDIDR = "dIDR"
)

var ErrInvalidAsset = errors.New("invalid ledger asset")

func IsSupportedAsset(asset string) bool {
	switch asset {
	case AssetIDR, AssetDIDR:
		return true
	default:
		return false
	}
}
