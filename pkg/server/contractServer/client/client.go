package client

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/blockchain"
	"github.com/korthochain/korthochain/pkg/config"
	"github.com/korthochain/korthochain/pkg/crypto"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/transaction"
	"github.com/korthochain/korthochain/pkg/txpool"
	"go.uber.org/zap"

	"github.com/btcsuite/btcutil/base58"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type Client struct {
	Bc  blockchain.Blockchains
	Tp  *txpool.Pool
	Cfg *config.CfgInfo
}

//new Client
func New(bc blockchain.Blockchains, tp *txpool.Pool, cfg *config.CfgInfo) *Client {
	return &Client{Bc: bc, Tp: tp, Cfg: cfg}
}

//cal contract
func (c *Client) ContractCall(origin string, contractAddr string, callInput, value string) (string, string, error) {
	return c.Bc.CallSmartContract(contractAddr, origin, callInput, value)
}

//Get from Balance
func (c *Client) GetBalance(from string) (uint64, error) {
	addr, err := address.StringToAddress(from)
	if err != nil {
		return 0, err
	}
	return c.Bc.GetBalance(addr)
}

//get block by hash
func (c *Client) GetBlockByHash(hash string) (*block.Block, error) {
	h, err := transaction.StringToHash(blockchain.Check0x(hash))
	if err != nil {
		return nil, err
	}
	return c.Bc.GetBlockByHash(h)
}

//get block by Number
func (c *Client) GetBlockByNumber(num uint64) (*block.Block, error) {
	return c.Bc.GetBlockByHeight(num)
}

//Get Transaction By Hash
func (c *Client) GetTransactionByHash(hash string) (*transaction.FinishedTransaction, error) {
	h, err := transaction.StringToHash(blockchain.Check0x(hash))
	if err != nil {
		return nil, err
	}
	ftx, err := c.Bc.GetTransactionByHash(h)
	if err != nil {
		stx, err := c.Tp.GetTxByHash(blockchain.Check0x(hash))
		if err != nil {
			return nil, err
		}
		return &transaction.FinishedTransaction{SignedTransaction: *stx}, nil
	}
	return ftx, nil
}

//Get Code by contract Address
func (c *Client) GetCode(contractAddr string) string {
	return common.Bytes2Hex(c.Bc.GetCode(contractAddr))
}

//Get address Nonce
func (c *Client) GetNonce(from string) (uint64, error) {
	addr, err := address.StringToAddress(from)
	if err != nil {
		return 0, err
	}
	return c.Bc.GetNonce(addr)
}

//Send signed Transaction
func (c *Client) SendRawTransaction(rawTx string) (string, error) {
	arr := strings.Split(rawTx, "0x0x0x")
	var msgHash []byte
	var ktoFrom string
	if len(arr) > 1 {
		arrmk := strings.Split(arr[1], "0x0x")
		if len(arrmk) > 1 {
			msgHash, _ = hex.DecodeString(arrmk[0])

			kaddr, _ := hex.DecodeString(arrmk[1])
			ktoFrom = base58.Encode(kaddr)
		}
		rawTx = arr[0]
	}

	decTX, err := hexutil.Decode(rawTx)
	if err != nil {
		return "", err
	}
	var tx types.Transaction

	err = rlp.DecodeBytes(decTX, &tx)
	if err != nil {
		return "", err
	}

	signer := types.NewEIP2930Signer(tx.ChainId())
	mas, err := tx.AsMessage(signer, nil)
	if err != nil {
		return "", err
	}

	logger.InfoLogger.Printf("chainserver sendRawTransaction:{mas.From:%v,to:%v,amount:%v,nounce:%v,hash:%v,gas:%v,gasPrice:%v,txType:%v,chainId:%v,tx lenght:%v}\n", mas.From(), tx.To(), tx.Value(), tx.Nonce(), tx.Hash(), tx.Gas(), tx.GasPrice(), tx.Type(), tx.ChainId(), len(tx.Data()))

	return c.sendRawTransaction(mas.From().Hex(), rawTx, ktoFrom, msgHash)
}

//send eth signed transaction
func (g *Client) sendRawTransaction(EthFrom, EthData, KtoFrom string, MsgHash []byte) (string, error) {
	ethFrom := common.HexToAddress(EthFrom)
	ethData := EthData

	if len(ethData) <= 0 {
		return "", fmt.Errorf("Wrong eth signed data:%s", fmt.Errorf("ethData length[%v] <= 0", len(ethData)).Error())
	}

	var from, to address.Address
	var evm transaction.EvmContract
	var amount, gasPrice, gasLimit uint64
	var tag transaction.TransactionType
	var signType crypto.SigType

	if len(KtoFrom) > 0 && len(MsgHash) > 0 {
		f, err := address.NewAddrFromString(KtoFrom)
		if err != nil {
			return "", err
		}
		from = f
		signType = crypto.TypeED25519
		evm.MsgHash = MsgHash
	} else {
		addr, err := address.StringToAddress(EthFrom)
		if err != nil {
			return "", fmt.Errorf("eth addr to kto addr error:%s", err.Error())
		}
		from = addr
		signType = crypto.TypeSecp256k1
	}

	ethTx, err := transaction.DecodeEthData(ethData)
	if err != nil {
		return "", fmt.Errorf("decodeData error:%s", err.Error())
	}
	if ethTx.ChainId().Int64() != g.Cfg.ChainCfg.ChainId {
		return "", fmt.Errorf("error chain ID: %v", ethTx.ChainId().Int64())
	}
	evm.EthData = EthData

	if len(ethTx.Data()) > 0 {
		to = address.ZeroAddress
		amount = 0
		if ethTx.To() == nil {
			evm.Origin = ethFrom
			evm.CreateCode = ethTx.Data()
			evm.Operation = blockchain.CREATECONTRACT
		} else {
			evm.ContractAddr = *ethTx.To()
			evm.CallInput = ethTx.Data()
			evm.Operation = blockchain.CALLCONTRACT
			evm.Origin = ethFrom
		}
		tag = transaction.EvmContractTransaction
	} else {
		ethTo := *ethTx.To()
		addr, err := address.StringToAddress(ethTo.Hex())
		if err != nil {
			return "", fmt.Errorf("eth addr to kto addr error:%s", err.Error())
		}
		to = addr

		deci := blockchain.Uint64ToBigInt(blockchain.ETHDECIMAL)
		value := ethTx.Value().Div(ethTx.Value(), deci)
		amount = value.Uint64()
		tag = transaction.EvmKtoTransaction
		evm.Status = true
	}

	if ethTx.GasPrice().Uint64() == 0 {
		gasPrice = g.Cfg.ChainCfg.GasPrice
	} else {
		gasPrice = ethTx.GasPrice().Uint64()
	}

	if ethTx.Gas() == 0 {
		gasLimit = g.Cfg.ChainCfg.GasLimit
	} else {
		gasLimit = ethTx.Gas()
	}

	input, err := transaction.EncodeEvmData(&evm)
	if err != nil {
		return "", fmt.Errorf("EncodeEvmData error:%s", err.Error())
	}

	nonce, err := g.Bc.GetNonce(from)
	if err != nil {
		return "", fmt.Errorf("GetNonce error:%s,from:%v", err.Error(), from)
	}
	if nonce != ethTx.Nonce() {
		return "", fmt.Errorf("error: from [%v] nonce[%v] not equal ethTx.Nonce[%v]", from, nonce, ethTx.Nonce())
	}

	tx := &transaction.SignedTransaction{
		Transaction: transaction.Transaction{
			From:      from,
			To:        to,
			Amount:    amount,
			Nonce:     ethTx.Nonce(),
			GasLimit:  gasLimit,
			GasPrice:  gasPrice,
			GasFeeCap: gasLimit,
			Type:      tag,
			Input:     input,
		},
		Signature: crypto.Signature{
			SigType: signType,
			Data:    transaction.ParseEthSignature(&ethTx),
		},
	}
	err = g.Tp.Add(tx)
	if err != nil {
		return "", fmt.Errorf("add tx pool error:%s", err.Error())
	}

	logger.Info("rpc End SendEthSignedRawTransaction:", zap.String("hash", tx.HashToString()))
	return tx.HashToString(), nil
}

//Get Transaction Receipt by hash
func (c *Client) GetTransactionReceipt(hash string) (*transaction.FinishedTransaction, error) {
	h, err := transaction.StringToHash(blockchain.Check0x(hash))
	if err != nil {
		return nil, err
	}
	return c.Bc.GetTransactionByHash(h)
}

//GetStorageAt
func (c *Client) GetStorageAt(addr, hash string) string {
	return c.Bc.GetStorageAt(addr, hash).Hex()
}

//get Logs
func (c *Client) GetLogs(address string, fromB, toB uint64, topics []string, blockH string) []*types.Log {
	logs := c.Bc.GetLogs()
	var resLogs []*types.Log
	for _, log := range logs {
		if common.HexToHash(blockH) == log.BlockHash {
			resLogs = append(resLogs, log)
		} else if log.BlockNumber >= fromB && log.BlockNumber <= toB {
			if common.HexToAddress(address) == log.Address {
				resLogs = append(resLogs, log)
			}
		}
	}
	return resLogs
}

//Get Max BlockNumber
func (c *Client) GetMaxBlockNumber() (uint64, error) {
	return c.Bc.GetMaxBlockHeight()
}
