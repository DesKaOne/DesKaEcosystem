package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
)

var (
	ErrInvalidKey        = errors.New("invalid ed25519 key")
	ErrInvalidPrivateKey = errors.New("invalid ed25519 private key")
	ErrInvalidPublicKey  = errors.New("invalid ed25519 public key")
)

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

// Ed25519Signer is the development signing implementation.
// The protocol-level algorithm remains subject to crypto-spec-v0.1 freeze.
type Ed25519Signer struct {
	privateKey ed25519.PrivateKey
}

func NewEd25519Signer(privateKey []byte) (*Ed25519Signer, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, ErrInvalidPrivateKey
	}
	key := make([]byte, len(privateKey))
	copy(key, privateKey)
	return &Ed25519Signer{privateKey: ed25519.PrivateKey(key)}, nil
}

func GenerateEd25519Signer() (*Ed25519Signer, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return NewEd25519Signer(privateKey)
}

func (s *Ed25519Signer) PublicKey() []byte {
	if s == nil || len(s.privateKey) != ed25519.PrivateKeySize {
		return nil
	}
	publicKey := s.privateKey.Public().(ed25519.PublicKey)
	out := make([]byte, len(publicKey))
	copy(out, publicKey)
	return out
}

func (s *Ed25519Signer) Sign(message []byte) ([]byte, error) {
	if s == nil || len(s.privateKey) != ed25519.PrivateKeySize {
		return nil, ErrInvalidPrivateKey
	}
	return ed25519.Sign(s.privateKey, message), nil
}

func (k *Ed25519KeyPair) Sign(message []byte) ([]byte, error) {
	if k == nil || len(k.PrivateKey) != ed25519.PrivateKeySize {
		return nil, ErrInvalidKey
	}
	return ed25519.Sign(k.PrivateKey, message), nil
}

func VerifyEd25519(message, signature, publicKey []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(publicKey), message, signature)
}
