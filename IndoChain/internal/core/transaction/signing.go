package transaction

import "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"

const transactionSigningDomain = "INDOCHAIN-TX"

// SigningBytesWithDomain returns development signing bytes with domain separation.
func SigningBytesWithDomain(tx Transaction) ([]byte, error) {
	raw, err := SigningBytes(tx)
	if err != nil {
		return nil, err
	}
	return crypto.DomainSeparatedMessage(transactionSigningDomain, uint16(tx.Version), raw), nil
}

// Sign creates a development Ed25519 signature for the transaction.
func Sign(tx Transaction, signer *crypto.Ed25519Signer) ([]byte, error) {
	signingBytes, err := SigningBytesWithDomain(tx)
	if err != nil {
		return nil, err
	}
	return signer.Sign(signingBytes)
}

// VerifySignature verifies a transaction signature against the supplied public key.
func VerifySignature(tx Transaction, signature, publicKey []byte) (bool, error) {
	signingBytes, err := SigningBytesWithDomain(tx)
	if err != nil {
		return false, err
	}
	return crypto.VerifyEd25519(signingBytes, signature, publicKey), nil
}
