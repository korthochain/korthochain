package ed25519

import (
	ed "crypto/ed25519"
	"crypto/rand"
	"errors"

	"github.com/korthochain/korthochain/pkg/crypto"
	"github.com/korthochain/korthochain/pkg/crypto/sigs"
)

type ED25519Signer struct{}

var _ sigs.SigShim = &ED25519Signer{}

func (s *ED25519Signer) Generate() ([]byte, error) {
	_, privateKey, err := ed.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return privateKey[:], nil
}

func (s *ED25519Signer) Verify(sig, publicKey, msg []byte) error {
	if len(publicKey) != ed.PublicKeySize || len(sig) != ed.SignatureSize {
		return errors.New("ed25519 signature failed to verify")
	}

	if !ed.Verify(publicKey, msg, sig) {
		return errors.New("ed25519 signature failed to verify")
	}

	return nil
}

func (s *ED25519Signer) ToPublic(priv []byte) ([]byte, error) {
	if len(priv) != ed.PrivateKeySize {
		return nil, errors.New("ed25519 signature invalid private key")
	}
	privateKey := make(ed.PrivateKey, ed.PrivateKeySize)
	copy(privateKey, priv[:ed.PrivateKeySize])
	return privateKey.Public().(ed.PublicKey)[:], nil
}

func (s *ED25519Signer) Sign(priv, msg []byte) ([]byte, error) {
	if len(priv) != ed.PrivateKeySize {
		return nil, errors.New("ed25519 signature invalid private key")
	}
	privateKey := make(ed.PrivateKey, ed.PrivateKeySize)
	copy(privateKey, priv[:ed.PrivateKeySize])
	return ed.Sign(privateKey, msg), nil
}

func init() {
	sigs.RegisterSignature(crypto.TypeED25519, new(ED25519Signer))
}
