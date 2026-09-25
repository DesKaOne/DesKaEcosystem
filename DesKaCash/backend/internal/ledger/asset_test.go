package ledger

import "testing"

func TestSupportedAssets(t *testing.T) {
	for _, asset := range []Asset{AssetIDR, AssetDIDR} {
		if !IsSupportedAsset(asset) {
			t.Fatalf("expected %q to be supported", asset)
		}
	}
}

func TestUnsupportedAsset(t *testing.T) {
	if IsSupportedAsset(Asset("USD")) {
		t.Fatal("USD must not be a supported ledger asset")
	}
}
