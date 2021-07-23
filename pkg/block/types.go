
package block

import (
	"math/big"
)

// Block Struct
type Block struct {
	Height       uint64   `json:"height"`     //当前块号
	PrevHash     []byte   `json:"prevHash"`   //上一块的hash json:"prevBlockHash --> json:"prevHash
	Hash         []byte   `json:"hash"`       //当前块hash
	Transactions []byte   `json:"hash"`       //交易数据
	Root         []byte   `json:"root"`       //Txhash 默克根
	SnapRoot     []byte   `json:"root"`       //默克根
	Version      uint64   `json:"version"`    //版本号
	Timestamp    uint64   `json:"timestamp"`  //时间戳
	Miner        []byte   `json:"miner"`      //矿工地址
	Difficulty   *big.Int `json:"difficulty"` //难度
	Nonce        uint64   `json:"nonce"`      //区块nonce
	GasLimit     uint64   `json:"gasLimit"`
	GasUsed      uint64   `json:"gasUsed"`
}
