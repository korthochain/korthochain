package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/korthochain/korthochain/pkg/blockchain"
	"github.com/korthochain/korthochain/pkg/config"
	"github.com/korthochain/korthochain/pkg/server/contractServer/client"
	"github.com/korthochain/korthochain/pkg/txpool"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const ETHKTODIC = 10000000

// Server struct
type Server struct {
	Bc  blockchain.Blockchains
	Tp  *txpool.Pool
	cli client.Clients
	Cfg *config.CfgInfo
}

type params struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Gas      string `json:"gas"`
	GasPrice string `json:"gasPrice"`
	Value    string `json:"value"`
	Data     string `json:"data"`
}

type responseBody struct {
	JsonRPC string      `json:"jsonrpc"`
	Id      interface{} `json:"id"`
	Result  interface{} `json:"result"`
}

type responseErr struct {
	JsonRPC string      `json:"jsonrpc"`
	Id      interface{} `json:"id"`
	Error   *ErrorBody  `json:"error"`
}

type ErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

type Transaction struct {
	BlockHash        common.Hash    `json:"blockHash"`
	BlockNumber      string         `json:"blockNumber"`
	From             common.Address `json:"from"`
	Gas              string         `json:"gas"`
	GasPrice         string         `json:"gasPrice"`
	Hash             common.Hash    `json:"hash"`
	Input            string         `json:"input"`
	Nonce            string         `json:"nonce"`
	To               common.Address `json:"to"`
	TransactionIndex string         `json:"transactionIndex"`
	Value            string         `json:"value"`
	V                string         `json:"v"`
	R                common.Hash    `json:"r"`
	S                common.Hash    `json:"S"`
}

type responseTransaction struct {
	JsonRPC string       `json:"jsonrpc"`
	Id      interface{}  `json:"id"`
	Result  *Transaction `json:"result"`
}

type TransactionReceipt struct {
	BlockHash         common.Hash    `json:"blockHash"`
	BlockNumber       string         `json:"blockNumber"`
	ContractAddress   common.Address `json:"contractAddress"`
	CumulativeGasUsed string         `json:"cumulativeGasUsed"`
	From              common.Address `json:"from"`
	GasUsed           string         `json:"gasUsed"`
	Logs              []*types.Log   `json:"logs"`
	LogsBloom         types.Bloom    `json:"logsBloom"`
	Status            string         `json:"status"`
	To                common.Address `json:"to"`

	TransactionHash  common.Hash `json:"transactionHash"`
	TransactionIndex string      `json:"transactionIndex"`

	Root common.Hash `json:"root"`
}

type responseReceipt struct {
	JsonRPC string              `json:"jsonrpc"`
	Id      interface{}         `json:"id"`
	Result  *TransactionReceipt `json:"result"`
}

type Block struct {
	Difficulty   string         `json:"difficulty"`
	ExtraData    string         `json:"extraData"`
	GasLimit     string         `json:"gasLimit"`
	GasUsed      string         `json:"gasUsed"`
	Hash         common.Hash    `json:"hash"`
	LogsBloom    string         `json:"logsBloom"`
	Miner        common.Address `json:"miner"`
	MixHash      string         `json:"mixHash"`
	Nonce        string         `json:"nonce"`
	Number       string         `json:"number"`
	ParentHash   common.Hash    `json:"parentHash"`
	TimeStamp    string         `json:"timestamp"`
	Transactions interface{}    `json:"transactions"`
}

type responseBlock struct {
	JsonRPC string      `json:"jsonrpc"`
	Id      interface{} `json:"id"`
	Result  *Block      `json:"result"`
}

type reqGetLog struct {
	FromBlock string   `json:"fromBlock"`
	ToBlock   string   `json:"toBlock"`
	Address   string   `json:"address"`
	Topics    []string `json:"topics"`
	BlockHash string   `json:"blockhash"`
}

var (
	ETH_CHAINID               string = "eth_chainId"
	NET_VERSION               string = "net_version"
	NET_LISTENING             string = "net_listening"
	ETH_SENDTRANSACTION       string = "eth_sendTransaction"
	ETH_CALL                  string = "eth_call"
	ETH_BLOCKNUMBER           string = "eth_blockNumber"
	ETH_GETBALANCE            string = "eth_getBalance"
	ETH_GETBLOCKBYHASH        string = "eth_getBlockByHash"
	ETH_GETBLOCKBYNUMBER      string = "eth_getBlockByNumber"
	ETH_GETTRANSACTIONBYHASH  string = "eth_getTransactionByHash"
	ETH_GASPRICE              string = "eth_gasPrice"
	EHT_GETCODE               string = "eth_getCode"
	ETH_GETTRANSACTIONCOUNT   string = "eth_getTransactionCount"
	ETH_ESTIMATEGAS           string = "eth_estimateGas"
	ETH_SENDRAWTRANSACTION    string = "eth_sendRawTransaction"
	ETH_GETTRANSACTIONRECEIPT string = "eth_getTransactionReceipt"
	ETH_GETLOGS               string = "eth_getLogs"
	ETH_GETSTORAGEAT          string = "eth_getStorageAt"
	ETH_SIGNTRANSACTION       string = "eth_signTransaction"
	ETH_ACCOUNTS              string = "eth_accounts"
	PERSONAL_UNLOCKACCOUNT    string = "personal_unlockAccount"

	WEB3_CLIENTVERSION string = "web3_clientVersion"
)

func getString(mp map[string]interface{}, k string) (string, error) {
	v, ok := mp[k]
	if !ok {
		return "", errors.New(fmt.Sprintf("'%s' not exist", k))
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	return "", errors.New(fmt.Sprintf("'%s' not string", k))
}

func getValue(mp map[string]interface{}, k string) (interface{}, error) {
	v, ok := mp[k]
	if !ok {
		return 0, errors.New(fmt.Sprintf("'%s' not exist", k))
	}
	//log.Printf("value type %T,value:%v\n", v, v)
	return v, nil
}

func getParam(mp map[string]interface{}) ([]interface{}, error) {
	v, ok := mp["params"]
	if !ok {
		return nil, errors.New(fmt.Sprintf("'%s' not exist", "params"))
	}
	return v.([]interface{}), nil
}

func responseErrFunc(code int, jsonRpc string, id interface{}, msg string) []byte {
	Err := &ErrorBody{Code: code, Message: msg}
	resp, err := json.Marshal(responseErr{JsonRPC: jsonRpc, Id: id, Error: Err})
	if err != nil {
		return []byte(errorMessage("eth_sendTransaction Marshal", err))
	} else {
		return resp
	}
}

func stringToHex(s string) string {
	return "0x" + s
}

func uint64ToHexString(val uint64) string {
	return stringToHex(fmt.Sprintf("%X", val))
}

func hexToUint64(hxs string) (uint64, error) {
	if len(hxs) > 2 {
		if hxs[:2] == "0x" {
			hxs = hxs[2:]
		}
	}
	n, err := strconv.ParseUint(hxs, 16, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

var (
	JsonMarshalErr   int = -5001
	JsonUnmarshalErr int = -5002
	ParameterErr     int = -5003
	IoutilErr        int = -5004
	UnkonwnErr       int = -5005
)

var (
	SUCCESS = "0x1"
	FAILED  = "0x0"
)

func errorMessage(msg string, err error) string {
	return fmt.Errorf("%v error: %v", msg, err.Error()).Error()
}
