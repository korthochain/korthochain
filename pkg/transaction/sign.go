package transaction

import (
	"errors"
	"fmt"

	"github.com/korthochain/korthochain/pkg/crypto"
	"github.com/korthochain/korthochain/pkg/crypto/sigs"
	_ "github.com/korthochain/korthochain/pkg/crypto/sigs/ed25519"
	_ "github.com/korthochain/korthochain/pkg/crypto/sigs/secp"
)

func (tx *Transaction) MutilSign(fromPriv, toPriv []byte) (*SignedTransaction, error) {
	if len(fromPriv) == 0 {
		return nil, fmt.Errorf("invalid from privte key")
	}

	if len(toPriv) == 0 {
		return nil, fmt.Errorf("invalid from privte key")
	}

	fsign, err := sigs.Sign(tx.From.Protocol(), fromPriv, tx.SignHash())
	if err != nil {
		return nil, err
	}

	tsign, err := sigs.Sign(tx.To.Protocol(), toPriv, tx.SignHash())
	if err != nil {
		return nil, err
	}

	mutilSign := crypto.Signature{SigType: crypto.TypeMutil, Data: append(fsign.Data, tsign.Data...)}

	return &SignedTransaction{Transaction: *tx, Signature: mutilSign}, nil
}

func (st *SignedTransaction) VerifySign() error {
	switch st.Type {
	case TransferTransaction, PledgeTrasnaction, PledgeBreakTransaction, WithdrawToEthTransaction:
		if err := sigs.Verify(&st.Signature, st.Caller().Payload(), st.SignHash()); err != nil {
			if err != nil {
				return err
			}
		}
	case LockTransaction, UnlockTransaction:
		var l int
		if st.From.Protocol() == crypto.TypeED25519 {
			l = 64
		} else if st.From.Protocol() == crypto.TypeSecp256k1 {
			l = 65
		}

		if len(st.Signature.Data) < l {
			return fmt.Errorf("invalide mutil signature")
		}

		callerSign := &crypto.Signature{st.From.Protocol(), st.Signature.Data[:l]}
		if err := sigs.Verify(callerSign, st.Caller().Bytes(), st.SignHash()); err != nil {
			return err
		}

		receiverSign := &crypto.Signature{st.To.Protocol(), st.Signature.Data[l:]}
		if err := sigs.Verify(receiverSign, st.Receiver().Bytes(), st.SignHash()); err != nil {
			return err
		}
	case EvmContractTransaction, EvmKtoTransaction:
		evm, err := DecodeEvmData(st.Input)
		if err != nil {
			return err
		}
		if len(evm.MsgHash) > 0 {
			from := st.Caller()
			if !VerifyKtoSign(&from, evm.MsgHash, evm.EthData) {
				return errors.New("verify kto transaction failed!")
			}
		} else {
			if !VerifyEthSign(evm.EthData) {
				return errors.New("verify eth transaction failed!")
			}
		}
	}

	return nil
}
