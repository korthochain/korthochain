package secp

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	localcyrpto "github.com/korthochain/korthochain/pkg/crypto"
	"github.com/korthochain/korthochain/pkg/crypto/sigs"
)

type Secp256k1Signer struct{}

func (s *Secp256k1Signer) Generate() ([]byte, error) {
	priv, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}

	return crypto.FromECDSA(priv), nil
}

func (s *Secp256k1Signer) ToPublic(privBytes []byte) ([]byte, error) {
	priv, err := crypto.ToECDSA(privBytes)
	if err != nil {
		return nil, err
	}

	return crypto.FromECDSAPub(priv.Public().(*ecdsa.PublicKey)), nil
}

func (s *Secp256k1Signer) Sign(privBytes, msg []byte) ([]byte, error) {
	priv, err := crypto.ToECDSA(privBytes)
	if err != nil {
		return nil, err
	}

	return crypto.Sign(msg, priv)
}

func (s *Secp256k1Signer) Verify(signature, pubKey, sighash []byte) error {
	if len(pubKey) != 21 {
		return fmt.Errorf("signature verification failed")
	}

	pub, err := crypto.Ecrecover(sighash, signature)
	if err != nil {
		return err
	}

	// addr, err := crypto.DecompressPubkey(pub)
	// if err != nil {
	// 	return err
	// }

	maybePK, err := crypto.UnmarshalPubkey(pub)
	if err != nil {
		return err
	}

	maybeAddr := crypto.PubkeyToAddress(*maybePK)

	if !bytes.Equal(maybeAddr.Bytes(), pubKey[1:]) {
		return fmt.Errorf("signature verification failed")
	}

	if !crypto.VerifySignature(pub, sighash, signature[:len(signature)-1]) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

func init() {
	sigs.RegisterSignature(localcyrpto.TypeSecp256k1, new(Secp256k1Signer))
}
