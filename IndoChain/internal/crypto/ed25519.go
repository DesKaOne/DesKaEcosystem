package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
)

var ErrInvalidKey = errors.New("invalid ed25519 key")

type Ed25519KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

func GenerateEd25519KeyPair() (*Ed25519KeyPair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Ed25519KeyPair{PrivateKey: privateKey, PublicKey: publicKey}, nil
}

func NewEd25519KeyPair(seed []byte) (*Ed25519KeyPair, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, ErrInvalidKey
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := make([]byte, ed25519.PublicKeySize)
	copy(publicKey, privateKey[32:])
	return &Ed25519KeyPair{PrivateKey: privateKey, PublicKey: ed25519.PublicKey(publicKey)}, nil
}

func (k *Ed25519KeyPair) Sign(message []byte) ([]byte, error) {
	if k == nil || len(k.PrivateKey) != ed25519.PrivateKeySize {
		return nil, ErrInvalidKey
	}
	return ed25519.Sign(k.PrivateKey, message), nil
}

func VerifyEd25519(publicKey, message, signature []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(publicKey), message, signature)
}
