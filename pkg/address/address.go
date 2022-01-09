package address

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/korthochain/korthochain/pkg/crypto"
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
	ED25519PubKeySize    = 32
	Secp256k1PayloadSize = 21
)

const (
	AddressSize        = 44
	AddresPrefixSize   = 3
	SecpAddressSzie    = 44
	Ed25519AddressSzie = 47
)

const (
	MainnetPrefix = "Kto"
	TestnetPrefix = "otK"
)

const (
	UndefAddressString = ""
)

var GenesisAddress = "otK00000000000000000000000000000000000000000"

var Undef = Address{}
var CurrentNetWork = Testnet

var prefixSet = map[byte]string{
	Mainnet: MainnetPrefix,
	Testnet: TestnetPrefix,
}

func SetNetWork(networkType string) {
	nwt := strings.ToLower(networkType)
	switch nwt {
	case "mainnet":
		CurrentNetWork = Mainnet
	case "testnet":
		CurrentNetWork = Testnet
	default:
		CurrentNetWork = Testnet
	}

	GenesisAddress = prefixSet[CurrentNetWork] + GenesisAddress[3:]
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

	return newAddress(crypto.TypeED25519, pubkey)
}

func NewSecp256k1Addr(pub []byte) (Address, error) {
	secpPub, err := ethcrypto.UnmarshalPubkey(pub)
	if err != nil {
		return Undef, err
	}
	ethAddr := ethcrypto.PubkeyToAddress(*secpPub)
	return NewFromEthAddr(ethAddr)
}

func NewAddrFromString(str string) (Address, error) {
	return decode(str)
}

func NewFromBytes(addr []byte) (Address, error) {
	if len(addr) == 0 {
		return Undef, nil
	}

	if len(addr) < Secp256k1PayloadSize {
		return Undef, fmt.Errorf("invalid address bytes")
	}

	return Address{string(addr)}, nil
}

func NewFromEthAddr(eaddr common.Address) (Address, error) {
	addrBytes := eaddr.Bytes()
	return newAddress(crypto.TypeSecp256k1, addrBytes)
}

func newAddress(protocol crypto.SigType, payload []byte) (Address, error) {
	var buf []byte
	var err error
	switch protocol {
	case crypto.TypeED25519:
		buf = payload
	case crypto.TypeSecp256k1:
		buf = make([]byte, 21)
		buf[0] = crypto.TypeSecp256k1
		copy(buf[1:], payload)
	default:
		err = fmt.Errorf("unknown address type")
	}

	return Address{string(buf)}, err
}

func (a *Address) Protocol() crypto.SigType {
	if len(a.string) == 0 {
		return crypto.TypeUnknown
	}

	if a.string[0] == crypto.TypeSecp256k1 && len(a.string) == Secp256k1PayloadSize {
		return crypto.TypeSecp256k1
	} else if len(a.string) == ED25519PubKeySize {
		return crypto.TypeED25519
	}

	return crypto.TypeUnknown
}

func (a *Address) NewCommonAddr() (common.Address, error) {
	var err error
	var addr common.Address

	if *a == Undef {
		return common.Address{}, fmt.Errorf("undef address")
	}

	switch a.Protocol() {
	case crypto.TypeED25519:
		sha := sha1.New()
		sha.Write([]byte(a.String()))
		addr = common.HexToAddress(hex.EncodeToString(sha.Sum(nil)))
	case crypto.TypeSecp256k1:
		payload := a.Payload()
		addr = common.BytesToAddress(payload[1:])
	default:
		err = fmt.Errorf("unknown address type")
	}

	return addr, err
}

func encode(network NetWork, addr Address) (string, error) {

	if addr == Undef {
		return UndefAddressString, nil
	}

	prefix, ok := prefixSet[network]
	if !ok {
		return Undef.string, fmt.Errorf("unknown address network")
	}
	var err error
	var addrStr string
	switch addr.Protocol() {
	case crypto.TypeSecp256k1:
		buf := addr.Payload()
		addrStr = prefix + "0" + hex.EncodeToString(buf[1:])
	case crypto.TypeED25519:
		addrStr = prefix + addrcodec.EncodeAddr(addr.Payload())
	default:
		err = fmt.Errorf("unknown address protocol")
	}

	return addrStr, err
}

func decode(str string) (Address, error) {
	if len(str) < AddressSize {
		return Undef, fmt.Errorf("invalid address string length")
	}

	if str[:AddresPrefixSize] != prefixSet[CurrentNetWork] {
		return Undef, fmt.Errorf("unknow address network")
	}

	var err error
	var payload []byte
	var protocol crypto.SigType = crypto.TypeUnknown
	switch parseAddrStrProtocol(str[AddresPrefixSize:]) {
	case crypto.TypeSecp256k1:
		payload, err = hex.DecodeString(str[AddresPrefixSize+1:])
		if err != nil {
			return Undef, err
		}
		protocol = crypto.TypeSecp256k1
	case crypto.TypeED25519:
		payload = addrcodec.DecodeAddr(str[AddresPrefixSize:])
		protocol = crypto.TypeED25519
	default:
		return Undef, fmt.Errorf("unknow address type")
	}

	return newAddress(protocol, payload)
}

func parseAddrStrProtocol(str string) crypto.SigType {
	if str[0] == '0' && len(str) == SecpAddressSzie-AddresPrefixSize {
		return crypto.TypeSecp256k1
	} else if len(str) == Ed25519AddressSzie-AddresPrefixSize {
		return crypto.TypeED25519
	}

	return crypto.TypeUnknown
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

func StringToAddress(addr string) (Address, error) {
	if len(addr) < 4 {
		return ZeroAddress, fmt.Errorf("wrong address %v", addr)
	}

	if addr[:3] == MainnetPrefix || addr[:3] == TestnetPrefix {
		a, err := NewAddrFromString(addr)
		if err != nil {
			return ZeroAddress, err
		}
		return a, nil
	} else if addr[:2] == "0x" {
		a, err := NewFromEthAddr(common.HexToAddress(addr))
		if err != nil {
			return ZeroAddress, err
		}
		return a, nil
	}

	return ZeroAddress, fmt.Errorf("Unsupport address type:%v", addr)
}
