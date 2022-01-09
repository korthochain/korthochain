package client

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/korthochain/korthochain/pkg/transaction"

	"github.com/korthochain/korthochain/pkg/block"
)

type Clients interface {
	//call contract
	ContractCall(origin string, contractAddr string, callInput, value string) (string, string, error)
	//get balance by from
	GetBalance(from string) (uint64, error)
	//get block by hash
	GetBlockByHash(hash string) (*block.Block, error)
	//get block by number
	GetBlockByNumber(num uint64) (*block.Block, error)
	//get code by contract address
	GetCode(contractAddr string) string
	//get nonce by address
	GetNonce(addr string) (uint64, error)
	//get transaction by hash
	GetTransactionByHash(hash string) (*transaction.FinishedTransaction, error)
	//send signed transaction
	SendRawTransaction(rawTx string) (string, error)
	//get transaction receipt
	GetTransactionReceipt(hash string) (*transaction.FinishedTransaction, error)

	//get Storage by address and hash
	GetStorageAt(addr, hash string) string
	//get logs
	GetLogs(address string, fromB, toB uint64, topics []string, blockH string) []*types.Log
	//get max block number
	GetMaxBlockNumber() (uint64, error)
}
