package transaction

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	coreTps "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/logger"
	cbg "github.com/whyrusleeping/cbor-gen"
	"go.uber.org/zap"
)

type TransactionType = uint8

const (
	TransferTransaction TransactionType = iota
	LockTransaction
	UnlockTransaction
	MortgageTransaction
	CoinBaseTransaction
	BindingAddressTransaction
	EvmContractTransaction
	PledgeTrasnaction
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

//evm info
type EvmContract struct {
	EthTransaction *coreTps.Transaction `json:"evm signed data"`
	MsgHash        []byte               `json:"kto sign hash"`
	Operation      string               `json:"contract operation"`

	CreateCode []byte         `json:"create code"`
	Origin     common.Address `json:"origin"`

	ContractAddr common.Address `json:"contract address"`
	CallInput    []byte         `json:"call input code"`
	Ret          string         `json:"call ret"`
	Status       bool           `json:"call status"`

	Logs []*coreTps.Log `json:"evm logs"`
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

// errUpdate
func (tx *Transaction) IsTransferTrasnaction() bool {
	return tx.Type == TransferTransaction
}

// errUpdate
func (tx *Transaction) IsLockTransaction() bool {
	return tx.Type == LockTransaction
}

// errUpdate
func (tx *Transaction) IsUnlockTransaction() bool {
	return tx.Type == UnlockTransaction
}

// errUpdate
func (tx *Transaction) IsCoinBaseTransaction() bool {
	return tx.Type == CoinBaseTransaction
}

// Binding Address Transaction
func (tx *Transaction) IsBindingAddressTransaction() bool {
	return tx.Type == BindingAddressTransaction
}

// Evm Contract Transaction
func (tx *Transaction) IsEvmContractTransaction() bool {
	return tx.Type == EvmContractTransaction
}

// Pledge Trasnaction
func (tx *Transaction) IsPledgeTrasnaction() bool {
	return tx.Type == PledgeTrasnaction
}

func HashToString(hash []byte) string {
	return hex.EncodeToString(hash)
}

//Decode eth Transaction Data
func DecodeEthData(data string) (coreTps.Transaction, error) {
	decTx, err := hexutil.Decode(data)
	if err != nil {
		logger.Error("hexutil Decode error", zap.Error(err))
		return coreTps.Transaction{}, err
	}
	var ethTx coreTps.Transaction
	err = rlp.DecodeBytes(decTx, &ethTx)
	if err != nil {
		logger.Error("DecodeBytes error", zap.Error(err))
		return coreTps.Transaction{}, err
	}
	return ethTx, nil
}

//parse eth signature
func ParseEthSignature(ethtx *coreTps.Transaction) []byte {
	big8 := big.NewInt(8)
	v, r, s := ethtx.RawSignatureValues()
	v = new(big.Int).Sub(v, new(big.Int).Mul(ethtx.ChainId(), big.NewInt(2)))
	v.Sub(v, big8)

	var sign []byte
	sign = append(sign, r.Bytes()...)
	sign = append(sign, s.Bytes()...)
	sign = append(sign, byte(v.Uint64()-27))
	return sign
}

//It's a eth transaction struct signed by kto priv
func (tx *Transaction) VerifyKtoSign(kFrom *address.Address, msgHash []byte, ethtx *coreTps.Transaction) bool {
	sign := ParseEthSignature(ethtx)
	if len(sign) <= 64 {
		logger.Error("Verify Kto Sign lenght error", zap.Int("length:", len(sign)))
		return false
	}

	pubkey := kFrom.Payload()
	return ed25519.Verify(pubkey, msgHash, sign[:64])
}

//Verify Eth Signature
func (tx *Transaction) VerifyEthSign(ethtx *coreTps.Transaction) bool {
	sign := ParseEthSignature(ethtx)
	if len(sign) <= 64 {
		logger.Error("eth sign error", zap.Int("length:", len(sign)))
		return false
	}
	pub, err := crypto.Ecrecover(ethtx.Hash().Bytes(), sign)
	if err != nil {
		logger.Error("get pub key error", zap.Error(err))
		return false
	}
	return crypto.VerifySignature(pub, ethtx.Hash().Bytes(), sign[:64])
}

func EncodeEvmData(evm *EvmContract) ([]byte, error) {
	return json.Marshal(&evm)
}

func DecodeEvmData(input []byte) (*EvmContract, error) {
	var evm EvmContract
	err := json.Unmarshal(input, &evm)
	if err != nil {
		return nil, err
	}
	return &evm, nil
}
