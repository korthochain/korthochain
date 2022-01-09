package chainserver

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"

	"github.com/buaazp/fasthttprouter"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/blockchain"
	kevm "github.com/korthochain/korthochain/pkg/contract/evm"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/transaction"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func NewServer(bc blockchain.Blockchains, ip_port, grpcip string) *Server {
	return &Server{address: ip_port, bc: bc, r: fasthttprouter.New(), grpcIp: grpcip}
}

func (s *Server) RunServer() {
	s.r.POST("/", s.ServerHandler)
	if err := fasthttp.ListenAndServe(s.address, s.r.Handler); err != nil {
		logger.Error("failed to listen port", zap.Error(err), zap.String("port", s.address))
		os.Exit(-1)
	}
}

//server handler
func (s *Server) ServerHandler(ctx *fasthttp.RequestCtx) {
	var resBody []byte
	var errorCode int
	var errMessage string
	defer func() {
		if errorCode != Success {
			resStr := fmt.Sprintf(`{errorcode":%d,"errormsg":"%s"}`, errorCode, errMessage)
			resBody = []byte(resStr)
		}
		ctx.Write(resBody)
	}()

	ctx.Request.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Request.Header.Add("Access-Control-Allow-Headers", "Cotent-Type")
	ctx.Request.Header.Set("content-type", "application/json")

	reqBody := ctx.PostBody()
	reqData := make(map[string]interface{})
	if err := json.Unmarshal(reqBody, &reqData); err != nil {
		errorCode = ErrData
		errMessage = err.Error()
		return
	}

	method, err := getString(reqData, "method")
	if err != nil {
		errorCode = ErrData
		errMessage = err.Error()
		return
	}

	switch method {
	case "nonce":
		address, err := getString(reqData, "address")
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
		nonce, err := s.getNonce(address)
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}

		resBody, err = json.Marshal(&resultInfo{ErrorCode: 0, ErrorMsg: "ok", Result: nonce})
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
	case "balance":
		address, err := getString(reqData, "address")
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
		balance, err := s.getBalance(address)
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
		resBody, err = json.Marshal(&resultInfo{ErrorCode: 0, ErrorMsg: "ok", Result: balance})
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
	case "height":
		maxH, err := s.getMaxHeight()
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
		resBody, err = json.Marshal(&resultInfo{ErrorCode: 0, ErrorMsg: "ok", Result: maxH})
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
	case "transaction":
		hash, err := getString(reqData, "hash")
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}

		tx, err := s.getTransacionByHash(hash)
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}

		resBody, err = json.Marshal(&resultTransaction{ErrorCode: 0, ErrorMsg: "ok", Transaction: convertTransaction(tx)})
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}

	case "transactionReceipt":
		hash, err := getString(reqData, "hash")
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}

		tx, err := s.getTransactionReceipt(hash)
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
		resBody, err = json.Marshal(&resTransactionReceipt{ErrorCode: 0, ErrorMsg: "ok", Transaction: tx})
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
	case "block":
		var block *block.Block
		option, err := getString(reqData, "option")
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
		if option == "hash" {
			hash, err := getString(reqData, "hash")
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}
			b, err := s.getBlockByHash(hash)
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}
			block = b

		} else if option == "height" {
			strH, err := getString(reqData, "height")
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}

			hi, err := strconv.ParseUint(strH, 0, 64)
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}
			b, err := s.getBlockByHeight(hi)
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}
			block = b

		} else {
			b, err := s.getLatestBlock()
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}
			block = b
		}
		if block != nil {
			res, err := json.Marshal(&resultBlock{ErrorCode: 0, ErrorMsg: "ok", Block: convertBlock(block)})
			if err != nil {
				errorCode = -1
				errMessage = err.Error()
				return
			}
			resBody = res
		} else {
			errorCode = -1
			errMessage = err.Error()
		}
	case "token_10":
		option, err := getString(reqData, "option")
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
		symbol, err := getString(reqData, "symbol")
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
		var result uint64
		if option == "decimal" {
			deci, err := s.getTokenDemic_10(symbol)
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}
			result = deci
		} else if option == "balance" {
			addr, err := getString(reqData, "address")
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}
			a, err := address.StringToAddress(addr)
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}
			balance, err := s.getTokenBalance_10(a, symbol)
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}
			result = balance
		}
		resBody, err = json.Marshal(&resultInfo{ErrorCode: 0, ErrorMsg: "ok", Result: result})
		if err != nil {
			errorCode = -1
			errMessage = err.Error()
			return
		}
	case "token_20":
		option, err := getString(reqData, "option")
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
		contract, err := getString(reqData, "contract")
		if err != nil {
			errorCode = ErrData
			errMessage = err.Error()
			return
		}
		if option == "data" {
			data, err := s.getContractData(contract)
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}
			resBody, err = json.Marshal(&resultContract{ErrorCode: 0, ErrorMsg: "ok", Contract: data})
			if err != nil {
				errorCode = -1
				errMessage = err.Error()
				return
			}
		} else if option == "balance" {
			address, err := getString(reqData, "address")
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}

			bal, err := s.getBalanceToken_20(contract, address)
			if err != nil {
				errorCode = ErrData
				errMessage = err.Error()
				return
			}

			resBody, err = json.Marshal(&resultBalance{ErrorCode: 0, ErrorMsg: "ok", Balance: bal})
			if err != nil {
				errorCode = -1
				errMessage = err.Error()
				return
			}
		}
	default:
		errorCode = ErrData
		errMessage = fmt.Errorf("Error unsupport method:%v\n", method).Error()
		return
	}
	return
}

//get nonce,usage：{"method":"nonce","address":"0xd7863649D0A766D0022167b1fd4cfBF0743aA95"}
func (s *Server) getNonce(addr string) (uint64, error) {
	a, err := address.StringToAddress(addr)
	if err != nil {
		return 0, err
	}
	return s.bc.GetNonce(a)
}

//get balance,usage：{"method":"balance","address":"0xd7863649D0A766D0022167b1fd4cfBF0743aA95"}
func (s *Server) getBalance(addr string) (uint64, error) {
	a, err := address.StringToAddress(addr)
	if err != nil {
		return 0, err
	}
	return s.bc.GetBalance(a)
}

//get max block  height,usage：{"method":"height"}
func (s *Server) getMaxHeight() (uint64, error) {
	return s.bc.GetMaxBlockHeight()
}

//get transaction by hash,usage：{"method":"transaction","hash":"8b60152613a9f2fbea4c35c685e9e10104196685fca5a5af41282372cea3bf32"}
func (s *Server) getTransacionByHash(hash string) (*transaction.FinishedTransaction, error) {
	hs, err := transaction.StringToHash(blockchain.Check0x(hash))
	if err != nil {
		return nil, err
	}
	return s.bc.GetTransactionByHash(hs)
}

//get transaction by hash,usage：{"method":"transactionReceipt","hash":"8b60152613a9f2fbea4c35c685e9e10104196685fca5a5af41282372cea3bf32"}
func (s *Server) getTransactionReceipt(hash string) (*TransactionReceipt, error) {
	hs, err := transaction.StringToHash(blockchain.Check0x(hash))
	if err != nil {
		return nil, err
	}
	tx, err := s.bc.GetTransactionByHash(hs)
	if err != nil {
		return nil, err
	}

	var trp TransactionReceipt
	trp.TransactionHash = common.BytesToHash(tx.Hash())
	trp.BlockNumber = uint64ToHexString(tx.BlockNum)
	trp.GasUsed = uint64ToHexString(tx.GasUsed)
	trp.CumulativeGasUsed = trp.GasUsed

	trp.From = common.HexToAddress(tx.From.String())
	trp.To = common.HexToAddress(tx.From.String())

	e, _ := transaction.DecodeEvmData(tx.Input)
	if e != nil {
		trp.ContractAddress = e.ContractAddr
		if e.Operation == "create" || e.Operation == "Create" {
			trp.To = common.Address{}
		} else {
			trp.To = e.ContractAddr
		}

		var stat uint64
		if !e.Status {
			stat = 0
			trp.Status = "0x0"
		} else {
			stat = 1
			trp.Status = "0x1"
		}

		trp.Logs = make([]*types.Log, len(e.Logs))
		trp.Logs = e.Logs
		//set bloom
		receipt := &types.Receipt{
			Type:              uint8(tx.Type),
			Status:            stat,
			Logs:              e.Logs,
			TxHash:            common.BytesToHash(tx.Hash()),
			ContractAddress:   trp.ContractAddress,
			GasUsed:           tx.GasUsed,
			BlockHash:         common.BytesToHash([]byte{}),
			TransactionIndex:  0,
			BlockNumber:       new(big.Int).SetUint64(tx.BlockNum),
			PostState:         trp.Root.Bytes(),
			CumulativeGasUsed: tx.GasUsed,
		}
		receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

		trp.LogsBloom = receipt.Bloom
	} else {
		trp.Status = "0x1"
	}

	return &trp, nil
}

//get the latest block,usage：{"method":"block","option":"latest"}
func (s *Server) getLatestBlock() (*block.Block, error) {
	maxH, err := s.bc.GetMaxBlockHeight()
	if err != nil {
		return nil, err
	}
	return s.bc.GetBlockByHeight(maxH)
}

//get block by hash,usage：{"method":"block","option":"hash","hash":"efea83b34aaed42b29379c3396b0ef5c2f2b09733c3561174e71dfe2950bbb0c"}
func (s *Server) getBlockByHash(hash string) (*block.Block, error) {
	hs, err := transaction.StringToHash(blockchain.Check0x(hash))
	if err != nil {
		return nil, err
	}
	return s.bc.GetBlockByHash(hs)
}

//get block by height,usage：{"method":"block","option":"height","height":"88200"}
func (s *Server) getBlockByHeight(hit uint64) (*block.Block, error) {
	return s.bc.GetBlockByHeight(hit)
}

//get token-10 decimal,usage：{"method":"token_10","option":"decimal","symbol":"CM"}
func (s *Server) getTokenDemic_10(symbol string) (uint64, error) {
	return s.bc.GetTokenDemic([]byte(symbol))
}

//get token-10 decimal,usage：{"method":"token_10","option":"balance","symbol":"CM"}
func (s *Server) getTokenBalance_10(address address.Address, symbol string) (uint64, error) {
	return s.bc.GetTokenBalance(address, []byte(symbol))
}

//get get Contract Data,usage {"method":"token_20","option":"data","contract":"0x12584E92430cb23b246CAF46bF81b2Dd946883e9"}
func (s *Server) getContractData(contract string) (*contractData, error) {
	resN, _, err := s.bc.CallSmartContract(contract, "", NAME, "")
	if err != nil {
		return nil, err
	}
	name := kevm.ParseCallResultToString(resN)

	resBol, _, err := s.bc.CallSmartContract(contract, "", SYMBOL, "")
	if err != nil {
		return nil, err
	}
	symbol := kevm.ParseCallResultToString(resBol)

	if len(name) == 0 && len(symbol) == 0 {
		return nil, fmt.Errorf("contract address [%v] not exist.", contract)
	}

	resDeci, _, err := s.bc.CallSmartContract(contract, "", DECIMALS, "")
	if err != nil {
		return nil, err
	}
	decimal := kevm.ParseCallResultToBig(resDeci)

	resTotal, _, err := s.bc.CallSmartContract(contract, "", TOTALSUPPLY, "")
	if err != nil {
		return nil, err
	}
	totalSupply := kevm.ParseCallResultToBig(resTotal)

	if decimal == nil && totalSupply == nil {
		return nil, fmt.Errorf("contract address [%v] not exist.", contract)
	}

	return &contractData{Name: name, Symbol: symbol, Decimal: decimal, TotalSupply: totalSupply}, nil
}

//get token-20 balance,usage:{"method":"token_20","option":"balance","contract":"0x12584E92430cb23b246CAF46bF81b2Dd946883e9","address":"0x5C93F5Ba8CCe6626213b9b9Ae8C4da26e77F76A6"}
func (s *Server) getBalanceToken_20(contract, addr string) (*big.Int, error) {
	input := BalanceOfPerfix + blockchain.Check0x(addr)
	resTotal, _, err := s.bc.CallSmartContract(contract, "", input, "")
	if err != nil {
		return nil, err
	}
	return kevm.ParseCallResultToBig(resTotal), nil
}

//convert block to chainblock
func convertBlock(b *block.Block) *ChainBlock {
	ctx := make([]*ChainTransaction, len(b.Transactions))
	for i, ftx := range b.Transactions {
		ctx[i] = convertTransaction(ftx)
	}
	return &ChainBlock{
		Height:           b.Height,
		PrevHash:         transaction.HashToString(b.PrevHash),
		Hash:             transaction.HashToString(b.Hash),
		Transactions:     ctx,
		Root:             transaction.HashToString(b.Root),
		SnapRoot:         transaction.HashToString(b.SnapRoot),
		Version:          b.Version,
		Timestamp:        b.Timestamp,
		UsedTime:         b.UsedTime,
		Miner:            b.Miner.String(),
		Difficulty:       b.Difficulty,
		GlobalDifficulty: b.GlobalDifficulty,
		Nonce:            b.Nonce,
		GasLimit:         b.GasLimit,
		GasUsed:          b.GasUsed,
	}
}

//convertTransaction
func convertTransaction(ftx *transaction.FinishedTransaction) *ChainTransaction {
	return &ChainTransaction{
		Version:   ftx.Version,
		Type:      ftx.Type,
		From:      ftx.From.String(),
		To:        ftx.To.String(),
		Amount:    ftx.Amount,
		Nonce:     ftx.Nonce,
		GasLimit:  ftx.GasLimit,
		GasFeeCap: ftx.GasFeeCap,
		GasPrice:  ftx.GasPrice,
		Input:     transaction.HashToString(ftx.Input),
		Signature: transaction.HashToString(ftx.Signature.Data),
		GasUsed:   ftx.GasUsed,
		BlockNum:  ftx.BlockNum,
	}
}
