package transaction

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/korthochain/korthochain/pkg/address"

	cbg "github.com/whyrusleeping/cbor-gen"
)

type TransactionType = uint8

const (
	TransferTransaction TransactionType = iota
	LockTransaction
	UnlockTransaction
	MortgageTransaction
)

// Transaction
type Transaction struct {
	Version uint64
	Type    TransactionType
	From    address.Address
	To      address.Address
	Amount  uint64
	Nonce   uint64

	GasLimit  uint64
	GasFeeCap uint64
	GasPrice  uint64

	Input []byte
}

// Caller address
func (t *Transaction) Caller() address.Address {
	return t.From
}

// Receiver address
func (t *Transaction) Receiver() address.Address {
	return t.To
}

func (t *Transaction) AmountReceived() uint64 {
	return t.Amount
}

// SignHash required for signature
func (t *Transaction) SignHash() []byte {
	data, err := t.Serialize()
	if err != nil {
		//TODO: handling errors
		panic(err)
	}
	hash := sha256.Sum256(data)
	return hash[:]
}

// GasCap gas fee upper limit
func (t *Transaction) GasCap() uint64 {
	return t.GasFeeCap * t.GasPrice
}

// Serialize transaction in the cbor format
func (t *Transaction) Serialize() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if err := t.MarshalCBOR(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DeserializeTransaction deserializes binary data in cbor format into
// transaction, and returns an error if the data format is incorrect
func DeserializeTransaction(data []byte) (*Transaction, error) {
	tx := &Transaction{}
	buf := bytes.NewBuffer(data)
	if err := tx.UnmarshalCBOR(buf); err != nil {
		return nil, err
	}
	return tx, nil
}

func (t *Transaction) String() string {
	//TODO： string
	return ""
}

func (t *Transaction) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}

	scratch := make([]byte, 9)
	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, t.Version); err != nil {
		return err
	}

	{
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajByteString, 1); err != nil {
			return err
		}

		if _, err := w.Write([]byte{t.Type}); err != nil {
			return err
		}
	}

	if err := t.From.MarshalCBOR(w); err != nil {
		return err
	}

	if err := t.To.MarshalCBOR(w); err != nil {
		return err
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, t.Amount); err != nil {
		return err
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, t.Nonce); err != nil {
		return err
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, t.GasFeeCap); err != nil {
		return err
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, t.GasLimit); err != nil {
		return err
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, t.GasPrice); err != nil {
		return err
	}

	if err := cbg.WriteMajorTypeHeader(w, cbg.MajByteString, uint64(len(t.Input))); err != nil {
		return err
	}

	if _, err := w.Write(t.Input[:]); err != nil {
		return err
	}

	return nil
}

func (t *Transaction) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	scratch := make([]byte, 8)

	// Version
	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if maj != cbg.MajUnsignedInt {
		return fmt.Errorf("wrong type for uint64 field")
	}

	t.Version = extra

	{
		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}

		if maj != cbg.MajByteString {
			return fmt.Errorf("expected byte array")
		}

		if extra != 1 {
			return fmt.Errorf("t.Type: byte array length is wrong(%d)", extra)
		}

		var typeBytes = make([]byte, 1)

		if _, err := io.ReadFull(br, typeBytes[:]); err != nil {
			return err
		}

		t.Type = typeBytes[0]
	}

	// From
	{
		if err := t.From.UnmarshalCBOR(br); err != nil {
			return err
		}
	}

	// To
	{
		if err := t.To.UnmarshalCBOR(br); err != nil {
			return err
		}
	}

	// Amount
	{
		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}

		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}

		t.Amount = uint64(extra)
	}

	// Nonce
	{
		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}

		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}

		t.Nonce = uint64(extra)
	}

	// gas
	{
		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}

		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}

		t.GasFeeCap = uint64(extra)
	}

	{
		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}

		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}

		t.GasLimit = uint64(extra)
	}

	{
		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}

		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}

		t.GasPrice = uint64(extra)
	}

	// input
	{
		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}

		if extra > cbg.ByteArrayMaxLen {
			return fmt.Errorf("t.Input: byte array too large (%d)", extra)
		}

		if maj != cbg.MajByteString {
			return fmt.Errorf("expected byte array")
		}

		if extra > 0 {
			t.Input = make([]byte, extra)
		}

		if _, err := io.ReadFull(br, t.Input[:]); err != nil {
			return err
		}
	}

	return nil
}
