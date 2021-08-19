package sigs

import (
	"fmt"

	"github.com/korthochain/korthochain/pkg/crypto"
)

func Generate(sigType crypto.SigType) ([]byte, error) {
	s, ok := sigs[sigType]
	if !ok {
		return nil, fmt.Errorf("cannot generate private key with signature of unsupported type:%v", sigType)
	}

	return s.Generate()
}

func Sign(sigType crypto.SigType, priv, msg []byte) (*crypto.Signature, error) {
	signer, ok := sigs[sigType]
	if !ok {
		return nil, fmt.Errorf("cannot sign message with signature of unsupported type:%v", sigType)
	}

	signature, err := signer.Sign(priv, msg)
	if err != nil {
		return nil, err
	}

	return &crypto.Signature{
		SigType: sigType,
		Data:    signature,
	}, nil
}

func Verify(sig *crypto.Signature, pk, msg []byte) error {
	if sig == nil {
		return fmt.Errorf("signature is nil")
	}

	signer, ok := sigs[sig.SigType]
	if !ok {
		return fmt.Errorf("cannot verify message with signature of unsupported type:%v", sig.SigType)
	}

	return signer.Verify(sig.Data, pk, msg)
}

func ToPublic(sigType crypto.SigType, priv []byte) ([]byte, error) {
	signer, ok := sigs[sigType]
	if !ok {
		return nil, fmt.Errorf("cannot to public key with signature of unsupported type:%v", sigType)
	}

	return signer.ToPublic(priv)
}

type SigShim interface {
	Generate() ([]byte, error)
	Verify(signature, pubKey, data []byte) error
	ToPublic([]byte) ([]byte, error)
	Sign(priv, msg []byte) ([]byte, error)
}

var sigs map[crypto.SigType]SigShim

func RegisterSignature(sigType crypto.SigType, sigShim SigShim) {
	if sigs == nil {
		sigs = make(map[crypto.SigType]SigShim)
	}

	if _, ok := sigs[sigType]; ok {
		return
	}

	sigs[sigType] = sigShim
}
