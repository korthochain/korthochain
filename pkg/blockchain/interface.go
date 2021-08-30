// Package blockchain define the interface of blockchain and implement its object
package blockchain

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/transaction"
)

//Blockchains interface specification of blockchain
type Blockchains interface {
	// NewBlock create a new block for the blockchain
	NewBlock([]*transaction.SignedTransaction, address.Address) (*block.Block, error)
	// AddBlock add blocks to blockchain
	AddBlock(*block.Block) error
	// DeleteBlock delete some blocks from the blockchain
	DeleteBlock(uint64) error
	// GetBalance retrieves the balance from the given address or 0 if object not found
	GetBalance([]byte) (uint64, error)
	// GetFreezeBalance get the freeze balance of the address
	GetFreezeBalance([]byte) (uint64, error)
	// GetAvailableBalance get available balance of address
	GetAvailableBalance([]byte) (uint64, error)
	// GetNonce get the nonce of the address
	GetNonce([]byte) (uint64, error)
	// GetHash get the hash corresponding to the block height
	GetHash(uint64) ([]byte, error)
	// GetMaxBlockHeight get maximum block height
	GetMaxBlockHeight() (uint64, error)
	// GetBlockByHeight get the block corresponding to the block height
	GetBlockByHeight(uint64) (*block.Block, error)
	//get binding kto address by eth address
	GetBindingKtoAddress(ethAddr string) (address.Address, error)
	//get binding eth address by kto address
	GetBindingEthAddress(ktoAddr address.Address) (string, error)
	//call contract
	CallSmartContract(contractAddr, origin, callInput, value string) (string, error)
	//get code
	GetCode(contractAddr string) []byte
	//get storage by hash
	GetStorageAt(addr, hash string) common.Hash
	// GetTransactionByHash get the transaction corresponding to the transaction hash
	GetTransactionByHash([]byte) (*transaction.SignedTransaction, error)
}
