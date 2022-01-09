package crypto

import (
	"bytes"
	"fmt"
	"io"

	cbg "github.com/whyrusleeping/cbor-gen"
)

type SigType = byte

const (
	TypeSecp256k1 SigType = iota
	TypeED25519
	TypeMutil

	TypeUnknown = 255
)

const (
	SignatureMaxLength = 130 //(secp sign len) 65*2
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
	case TypeED25519:
		s.SigType = TypeED25519
	case TypeSecp256k1:
		s.SigType = TypeSecp256k1
	case TypeMutil:
		s.SigType = TypeMutil
	default:
		return fmt.Errorf("unkown signature type")
	}
	s.Data = buf[1:]

	return nil
}

// Serialize Signature in the cbor format
func (s *Signature) Serialize() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if err := s.MarshalCBOR(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DeserializeSignature deserializes binary data in cbor format into
// Signature, and returns an error if the data format is incorrect
func DeserializeSignature(data []byte) (*Signature, error) {
	s := &Signature{}
	buf := bytes.NewBuffer(data)
	if err := s.UnmarshalCBOR(buf); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Signature) String() string {
	//TODO: string
	return ""
}
