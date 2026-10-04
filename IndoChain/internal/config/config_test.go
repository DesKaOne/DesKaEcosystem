package config

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestDevnetMatchesGenesis(t *testing.T) {
	cfg := Devnet(); genesis := devnet.Default()
	if err := cfg.Validate(); err != nil { t.Fatal(err) }
	if cfg.NetworkProfile != genesis.NetworkProfile { t.Fatalf("profile = %q, want %q", cfg.NetworkProfile, genesis.NetworkProfile) }
	if cfg.ChainID != genesis.ChainID { t.Fatalf("chain ID = %d, want %d", cfg.ChainID, genesis.ChainID) }
	if cfg.ProtocolVersion != genesis.ProtocolVersion { t.Fatalf("protocol version = %d, want %d", cfg.ProtocolVersion, genesis.ProtocolVersion) }
}

func TestValidateRejectsMismatchedTransactionRules(t *testing.T) {
	cfg := Devnet(); cfg.Transaction.ChainID = types.ChainID(9999)
	if err := cfg.Validate(); err != ErrInvalidConfig { t.Fatalf("error = %v, want %v", err, ErrInvalidConfig) }
}

func TestBlockRulesCopiesPublicKey(t *testing.T) {
	cfg := Devnet(); key := []byte{1, 2, 3}
	rules, err := cfg.BlockRules(key); if err != nil { t.Fatal(err) }
	key[0] = 9
	if rules.ChainID != cfg.ChainID || rules.ProtocolVersion != cfg.ProtocolVersion { t.Fatal("block rules do not match chain config") }
	if rules.Transaction.PublicKey[0] != 1 { t.Fatal("public key was not copied") }
}