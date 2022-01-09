// Package blockchain define the interface of blockchain and implement its object
package blockchain

import (
	"github.com/ethereum/go-ethereum/common"
	evmtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/transaction"
)

//Blockchains interface specification of blockchain
type Blockchains interface {
	// NewBlock create a new block for the blockchain
	NewBlock([]*transaction.SignedTransaction, address.Address) (*block.Block, error)
	// AddBlock add blocks to blockchain
	AddUncleBlock(*block.Block) error
	// AddBlock add blocks to blockchain
	AddBlock(*block.Block) error
	// DeleteBlock delete some blocks from the blockchain
	DeleteBlock(uint64) error
	// DeleteUncleBlock delete some blocks from the blockchain
	DeleteUncleBlock(*block.Block) error
	// GetBalance retrieves the balance from the given address or 0 if object not found
	GetBalance(address.Address) (uint64, error)
	// GetAvailableBalance get available balance of address
	GetAvailableBalance(address.Address) (uint64, error)
	// GetNonce get the nonce of the address
	GetNonce(address.Address) (uint64, error)
	// GetHash get the hash corresponding to the block height
	GetHash(uint64) ([]byte, error)
	// GetMaxBlockHeight get maximum block height
	GetMaxBlockHeight() (uint64, error)
	// GetBlockByHeight get the block corresponding to the block height
	GetBlockByHeight(uint64) (*block.Block, error)
	// GetBlockByHash get block data through hash
	GetBlockByHash([]byte) (*block.Block, error)
	// GetAllFreezeBalance query all frozen quotas of an address
	GetAllFreezeBalance(address.Address) (uint64, error)
	// GetSingleFreezeBalance check an address and specify the frozen amount of the notary
	GetSingleFreezeBalance(address.Address, address.Address) (uint64, error)

	// GetTokenBalance get the balance of tokens
	GetTokenBalance(address.Address, []byte) (uint64, error)
	// GetFrozenTokenBal get the frozen balance of tokens
	GetFrozenTokenBal(address.Address, []byte) (uint64, error)
	// GetTokenRoot get token root
	GetTokenRoot(address.Address, string) ([]byte, error)
	// GetTokenDemic get token precision
	GetTokenDemic([]byte) (uint64, error)

	ReorganizeChain([][]byte, uint64) error
	Tip() (*block.Block, error)

	//call contract
	CallSmartContract(contractAddr, origin, callInput, value string) (string, string, error)
	//get code
	GetCode(contractAddr string) []byte
	//set code
	SetCode(contractAddr common.Address, code []byte)
	//get storage by hash
	GetStorageAt(addr, hash string) common.Hash
	// GetTransactionByHash get the transaction corresponding to the transaction hash
	GetTransactionByHash([]byte) (*transaction.FinishedTransaction, error)
	//Get logs
	GetLogs() []*evmtypes.Log
	DifficultDetection(b *block.Block) error
	CheckBlockRegular(b *block.Block) error
	GetBasePledge() (uint64, error)
}
