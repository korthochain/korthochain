// Package blockchain define the interface of blockchain and implement its object
package blockchain

import (
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/transaction"
)

//Blockchains interface specification of blockchain
type Blockchains interface {
	NewBlock([]*transaction.SignedTransaction, address.Address) (*block.Block, error)
	AddBlock(*block.Block) error
	DeleteBlock(uint64) error

	GetBalance([]byte) (uint64, error)
	GetFreezeBalance(address []byte) (uint64, error)
	GetNonce([]byte) (uint64, error)
	GetHash(uint64) ([]byte, error)
	GetMaxBlockHeight() (uint64, error)
	GetHeight() (uint64, error)
	GetBlockByHeight(uint64) (*block.Block, error)
	GetTransactionByHash([]byte) (*transaction.Transaction, error)
}
