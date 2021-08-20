package address

import (
	"fmt"
	"io"

	"github.com/korthochain/korthochain/pkg/util/addrcodec"

	cbg "github.com/whyrusleeping/cbor-gen"
)

// Address
type Address struct {
	string // string(public key)
}

// NetWork
type NetWork = byte

const (
	Mainnet NetWork = iota
	Testnet
)

const (
	ED25519PubKeySize = 32
)

const (
	AddressSize      = 47
	AddresPrefixSize = 3
)

const (
	MainnetPrefix = "Kto"
	TestnetPrefix = "otK"
)

const (
	UndefAddressString = ""
)

var Undef = Address{}
var CurrentNetWork = Testnet

var prefixSet = map[byte]string{
	Mainnet: MainnetPrefix,
	Testnet: TestnetPrefix,
}

func (a Address) String() string {
	str, err := encode(CurrentNetWork, a)
	if err != nil {
		panic(err)
	}

	return str
}

func (a Address) Bytes() []byte {
	return []byte(a.string)
}

func (a Address) Payload() []byte {
	return []byte(a.string)
}

func NewEd25519Addr(pubkey []byte) (Address, error) {
	if len(pubkey) != ED25519PubKeySize {
		return Undef, fmt.Errorf("invalid public key")
	}

	return Address{string(pubkey)}, nil
}

func NewAddrFromString(str string) (Address, error) {
	return decode(str)
}

func NewFromBytes(addr []byte) (Address, error) {
	if len(addr) == 0 {
		return Undef, nil
	}

	if len(addr) != ED25519PubKeySize {
		return Undef, fmt.Errorf("invalid address bytes")
	}

	return Address{string(addr)}, nil
}

func encode(network NetWork, addr Address) (string, error) {

	if addr == Undef {
		return UndefAddressString, nil
	}

	prefix, ok := prefixSet[network]
	if !ok {
		return Undef.string, fmt.Errorf("unknown address network")
	}

	return prefix + addrcodec.Encode([]byte(addr.string)), nil
}

func decode(str string) (Address, error) {
	if len(str) != AddressSize {
		return Undef, fmt.Errorf("invalid address string")
	}

	if str[:AddresPrefixSize] != prefixSet[CurrentNetWork] {
		return Undef, fmt.Errorf("unknow address network")
	}

	return Address{string(addrcodec.DecodeAddr(str[3:]))}, nil
}

func (a *Address) MarshalCBOR(w io.Writer) error {
	if a == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}

	if err := cbg.WriteMajorTypeHeader(w, cbg.MajByteString, uint64(len(a.string))); err != nil {
		return err
	}

	if _, err := io.WriteString(w, a.string); err != nil {
		return err
	}

	return nil
}

func (a *Address) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if maj != cbg.MajByteString {
		return fmt.Errorf("cbor type for address unmarshal was not byte string")
	}

	if extra != 32 {
		return fmt.Errorf("invalid number of bytes unmarshall for the address")
	}

	buf := make([]byte, extra)
	if _, err := io.ReadFull(br, buf); err != nil {
		return err
	}

	addr, err := NewFromBytes(buf)
	if err != nil {
		return err
	}

	if addr == Undef {
		return fmt.Errorf("cbor input should not contain empty addresses")
	}

	*a = addr
	return nil
}
