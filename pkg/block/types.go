package block

import (
	"bytes"
	"encoding/json"
	"math/big"

	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/storage/miscellaneous"
	"github.com/korthochain/korthochain/pkg/transaction"
	"golang.org/x/crypto/sha3"
)

// Block Struct
type Block struct {
	Height       uint64                           `json:"height"`       //当前块号
	PrevHash     []byte                           `json:"prevHash"`     //上一块的hash json:"prevBlockHash --> json:"prevHash
	Hash         []byte                           `json:"hash"`         //当前块hash
	Transactions []*transaction.SignedTransaction `json:"transactions"` //交易数据
	Root         []byte                           `json:"root"`         //Txhash 默克根
	SnapRoot     []byte                           `json:"root"`         //默克根
	Version      uint64                           `json:"version"`      //版本号
	Timestamp    uint64                           `json:"timestamp"`    //时间戳
	Miner        address.Address                  `json:"miner"`        //矿工地址
	Difficulty   *big.Int                         `json:"difficulty"`   //难度
	Nonce        uint64                           `json:"nonce"`        //区块nonce
	GasLimit     uint64                           `json:"gasLimit"`
	GasUsed      uint64                           `json:"gasUsed"`
}

//SetHash hash the block data
func (b *Block) SetHash() {
	heightBytes := miscellaneous.E64func(b.Height)
	txsBytes, _ := json.Marshal(b.Transactions)
	timeBytes := miscellaneous.E64func(uint64(b.Timestamp))
	blockBytes := bytes.Join([][]byte{heightBytes, b.PrevHash, txsBytes, timeBytes}, []byte{})
	hash := sha3.Sum256(blockBytes)
	b.Hash = hash[:]
}

// Serialize serialization using JSON format
// errUpdated
func (b *Block) Serialize() []byte {
	data, err := json.Marshal(b)
	if err != nil {
		return nil
	}
	return data
}

// Deserialize deserialization of JSON formatted block data
// errUpdated
func Deserialize(data []byte) (*Block, error) {
	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, err
	}
	return &block, nil
}
