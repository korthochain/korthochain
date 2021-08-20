package crypto

import (
	"fmt"
	"io"

	cbg "github.com/whyrusleeping/cbor-gen"
)

type SigType int

const (
	ED25519 SigType = iota
)

const (
	SignatureMaxLength = 64
)

type Signature struct {
	SigType SigType
	Data    []byte
}

func (s *Signature) MarshalCBOR(w io.Writer) error {
	if s == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}

	if err := cbg.WriteMajorTypeHeader(w, cbg.MajByteString, uint64(len(s.Data)+1)); err != nil {
		return err
	}

	if _, err := w.Write([]byte{byte(s.SigType)}); err != nil {
		return err
	}

	if _, err := w.Write(s.Data); err != nil {
		return err
	}

	return nil
}

func (s *Signature) UnmarshalCBOR(r io.Reader) error {

	maj, l, err := cbg.CborReadHeader(r)
	if err != nil {
		return err
	}

	if maj != cbg.MajByteString {
		return fmt.Errorf("not byte string")
	}

	if l > SignatureMaxLength+1 {
		return fmt.Errorf("string too long")
	}

	if l == 0 {
		return fmt.Errorf("string is empty")
	}

	buf := make([]byte, l)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return err
	}

	switch SigType(buf[0]) {
	case ED25519:
		s.SigType = ED25519
	default:
		return fmt.Errorf("unkown signature type")
	}
	s.Data = buf[1:]

	return nil
}

func (s *Signature) String() string {
	//TODO: string
	return ""
}
