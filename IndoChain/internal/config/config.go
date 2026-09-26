package config

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var ErrInvalidConfig = errors.New("invalid chain configuration")

// ChainConfig groups the chain profile and execution rules used by a node.
// Values are development configuration until protocol freeze.
type ChainConfig struct {
	NetworkProfile string
	ChainID types.ChainID
	ProtocolVersion types.ProtocolVersion
	Transaction transaction.ValidationRules
}

// Devnet returns the canonical development configuration for the Devnet profile.
func Devnet() ChainConfig {
	genesis := devnet.Default()
	return ChainConfig{
		NetworkProfile: genesis.NetworkProfile,
		ChainID: genesis.ChainID,
		ProtocolVersion: genesis.ProtocolVersion,
		Transaction: transaction.ValidationRules{
			ProtocolVersion: genesis.ProtocolVersion,
			ChainID: genesis.ChainID,
			RequireSender: true,
			RequireRecipient: true,
			RequireSignature: true,
			MinGasLimit: 1,
		},
	}
}

// Validate checks the internal consistency of the configuration.
func (c ChainConfig) Validate() error {
	if c.NetworkProfile == "" || c.ChainID == 0 || c.ProtocolVersion == 0 {
		return ErrInvalidConfig
	}
	if c.Transaction.ProtocolVersion != c.ProtocolVersion || c.Transaction.ChainID != c.ChainID {
		return ErrInvalidConfig
	}
	return nil
}

// BlockRules creates block execution rules for the configured chain.
// The public key remains an explicit input because the v0.1 transaction
// model does not yet carry a canonical sender public key field.
func (c ChainConfig) BlockRules(publicKey []byte) (block.ExecutionRules, error) {
	if err := c.Validate(); err != nil {
		return block.ExecutionRules{}, err
	}
	return block.ExecutionRules{
		ChainID: c.ChainID,
		ProtocolVersion: c.ProtocolVersion,
		Transaction: state.ExecutionRules{
			Validation: c.Transaction,
			PublicKey: append([]byte(nil), publicKey...),
		},
	}, nil
}