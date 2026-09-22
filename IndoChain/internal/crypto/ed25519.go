package crypto

import (
    "crypto/ed25519"
    "crypto/rand"
    "errors"
)

var (
    ErrInvalidPrivateKey = errors.New("invalid ed25519 private key")
    ErrInvalidPublicKey = errors.New("invalid ed25519 public key")
)

// Ed25519Signer is the development signing implementation.
// The protocol-level algorithm remains subject to crypto-spec-v0.1 freeze.
type Ed25519Signer struct { privateKey ed25519.PrivateKey }

func NewEd25519Signer(privateKey []byte) (*Ed25519Signer, error) {
    if len(privateKey) != ed25519.PrivateKeySize { return nil, ErrInvalidPrivateKey }
    key := make([]byte, len(privateKey)); copy(key, privateKey)
    return &Ed25519Signer{privateKey: ed25519.PrivateKey(key)}, nil
}

func GenerateEd25519Signer() (*Ed25519Signer, error) {
    _, privateKey, err := ed25519.GenerateKey(rand.Reader)
    if err != nil { return nil, err }
    return NewEd25519Signer(privateKey)
}

func (s *Ed25519Signer) PublicKey() []byte {
    publicKey := s.privateKey.Public().(ed25519.PublicKey)
    out := make([]byte, len(publicKey)); copy(out, publicKey); return out
}

func (s *Ed25519Signer) Sign(message []byte) ([]byte, error) {
    if s == nil || len(s.privateKey) != ed25519.PrivateKeySize { return nil, ErrInvalidPrivateKey }
    return ed25519.Sign(s.privateKey, message), nil
}

func VerifyEd25519(message, signature, publicKey []byte) bool {
    if len(publicKey) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize { return false }
    return ed25519.Verify(ed25519.PublicKey(publicKey), message, signature)
}