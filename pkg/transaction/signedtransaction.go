package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/crypto"

	cbg "github.com/whyrusleeping/cbor-gen"
)

type SignedTransaction struct {
	Transaction Transaction
	Signature   crypto.Signature
}

func (st *SignedTransaction) Caller() address.Address {
	return st.Transaction.Caller()
}

func (st *SignedTransaction) Nonce() uint64 {
	return st.Transaction.Nonce
}

func (st *SignedTransaction) GasCap() uint64 {
	return st.Transaction.GasCap()
}

func (st *SignedTransaction) Hash() []byte {
	data, err := st.Serialize()
	if err != nil {
		panic(err)
	}

	hash := sha256.Sum256(data)
	return hash[:]
}

func (st *SignedTransaction) Serialize() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if err := st.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func DeserializeSignaturedTransaction(data []byte) (*SignedTransaction, error) {
	st := &SignedTransaction{}

	if err := st.UnmarshalCBOR(bytes.NewReader(data)); err != nil {
		return nil, err
	}

	return st, nil
}

func (st *SignedTransaction) MarshalCBOR(w io.Writer) error {
	if st == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}

	if _, err := w.Write([]byte{130}); err != nil {
		return err
	}

	if err := st.Transaction.MarshalCBOR(w); err != nil {
		return err
	}

	if err := st.Signature.MarshalCBOR(w); err != nil {
		return err
	}

	return nil
}

func (st *SignedTransaction) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)
	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 2 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// st.Transaction
	if err := st.Transaction.UnmarshalCBOR(r); err != nil {
		return err
	}

	// st.Signature
	if err := st.Signature.UnmarshalCBOR(r); err != nil {
		return err
	}

	return nil
}

func (st *SignedTransaction) String() string {
	//TODO:string
	return fmt.Sprintf("{Transaction:%s,Signature:%s}", st.Transaction.String(), st.Signature.String())
}

func (st *SignedTransaction) GetTransaction() Transaction {
	return st.Transaction
}

func (st *SignedTransaction) HashToString() string {
	return hex.EncodeToString(st.Hash())
}
