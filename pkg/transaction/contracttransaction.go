package transaction

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	coreTps "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/logger"
	"go.uber.org/zap"
)

// Binding Address Transaction
func (tx *Transaction) IsBindingAddressTransaction() bool {
	return tx.Type == BindingAddressTransaction
}

// Evm Contract Transaction
func (tx *Transaction) IsEvmContractTransaction() bool {
	return tx.Type == EvmContractTransaction
}

// Evm Kto Transaction
func (tx *Transaction) IsKtoTransaction() bool {
	return tx.Type == EvmKtoTransaction
}

// Pledge Trasnaction
func (tx *Transaction) IsPledgeTrasnaction() bool {
	return tx.Type == PledgeTrasnaction
}

func (tx *Transaction) IsPledgeBreakTransaction() bool {
	return tx.Type == PledgeBreakTransaction
}

func HashToString(hash []byte) string {
	return hex.EncodeToString(hash)
}
func StringToHash(hash string) ([]byte, error) {
	return hex.DecodeString(hash)
}

//evm info
type EvmContract struct {
	EthData   string `json:"evm signed data"`
	MsgHash   []byte `json:"kto sign hash"`
	Operation string `json:"contract operation"`

	CreateCode []byte         `json:"create code"`
	Origin     common.Address `json:"origin"`

	ContractAddr common.Address `json:"contract address"`
	CallInput    []byte         `json:"call input code"`
	Ret          string         `json:"call ret"`
	Status       bool           `json:"call status"`

	Logs []*coreTps.Log `json:"evm logs"`
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
func VerifyKtoSign(kFrom *address.Address, msgHash []byte, ethdata string) bool {
	ethtx, err := DecodeEthData(ethdata)
	if err != nil {
		logger.Info("SendEthSignedTransaction decodeData error", zap.Error(err))
		return false
	}

	sign := ParseEthSignature(&ethtx)
	if len(sign) <= 64 {
		logger.Error("Verify Kto Sign lenght error", zap.Int("length:", len(sign)))
		return false
	}

	pubkey := kFrom.Payload()
	return ed25519.Verify(pubkey, msgHash, sign[:64])
}

//Verify Eth Signature
func VerifyEthSign(ethdata string) bool {
	ethtx, err := DecodeEthData(ethdata)
	if err != nil {
		logger.Info("SendEthSignedTransaction decodeData error", zap.Error(err))
		return false
	}

	sign := ParseEthSignature(&ethtx)
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

func DecodeKtoTxData(data []byte) (*FinishedTransaction, error) {
	var ftx FinishedTransaction
	err := json.Unmarshal(data, &ftx)
	if err != nil {
		logger.Error("gDecodeKtoTxData error", zap.Error(err))
		return nil, err
	}
	return &ftx, nil
}
