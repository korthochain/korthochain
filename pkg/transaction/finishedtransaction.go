package transaction

import (
	"bytes"
	"fmt"
	"io"

	cbg "github.com/whyrusleeping/cbor-gen"
)

type FinishedTransaction struct {
	SignedTransaction
	GasUsed  uint64
	BlockNum uint64
}

func (t *FinishedTransaction) GetGasUsed() uint64 {
	return t.GasUsed
}

func (ft *FinishedTransaction) MarshalCBOR(w io.Writer) error {
	if ft == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}

	if _, err := w.Write([]byte{130}); err != nil {
		return err
	}

	if err := ft.SignedTransaction.MarshalCBOR(w); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// GasUsed
	{
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, ft.GasUsed); err != nil {
			return err
		}
	}

	// BlockNum
	{
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, ft.BlockNum); err != nil {
			return err
		}
	}
	return nil
}

func (ft *FinishedTransaction) UnmarshalCBOR(r io.Reader) error {
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

	// ft.Transaction
	if err := ft.SignedTransaction.UnmarshalCBOR(r); err != nil {
		return err
	}

	// GasUsed
	{
		maj, extra, err = cbg.CborReadHeader(r)
		if err != nil {
			return err
		}

		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}

		ft.GasUsed = extra
	}

	// BlockNum
	{
		maj, extra, err := cbg.CborReadHeader(r)
		if err != nil {
			return err
		}

		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}

		ft.BlockNum = extra
	}

	return nil
}

func (st *FinishedTransaction) Serialize() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if err := st.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func DeserializeFinishedTransaction(data []byte) (*FinishedTransaction, error) {
	st := &FinishedTransaction{}

	if err := st.UnmarshalCBOR(bytes.NewReader(data)); err != nil {
		return nil, err
	}

	return st, nil
}
