package api

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goinggo/mapstructure"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/blockchain"
	"github.com/korthochain/korthochain/pkg/config"
	"github.com/korthochain/korthochain/pkg/server/contractServer/client"
	"github.com/korthochain/korthochain/pkg/transaction"
	"github.com/korthochain/korthochain/pkg/txpool"
)

//New Server
func NewMetamaskServer(bc blockchain.Blockchains, tp *txpool.Pool, cfg *config.CfgInfo) *Server {
	clit := client.New(bc, tp, cfg)
	return &Server{Cfg: cfg, cli: clit}
}

//Handle Request
func (s *Server) HandRequest(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("content-type", "application/json")

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		resE := responseErrFunc(IoutilErr, "", 0, errorMessage("ioutil ReadAll", err))
		w.Write(resE)
		return
	}

	reqData := make(map[string]interface{})
	if err := json.Unmarshal(body, &reqData); err != nil {
		resE := responseErrFunc(JsonUnmarshalErr, "", 0, err.Error())
		w.Write(resE)
		return
	}

	method, err := getString(reqData, "method")
	if err != nil {
		resE := responseErrFunc(ParameterErr, "", 0, errorMessage("get method", err))
		w.Write(resE)
		return
	}

	jsonrpc, err := getString(reqData, "jsonrpc")
	if err != nil {
		resE := responseErrFunc(ParameterErr, "", 0, errorMessage("get jsonrpc", err))
		w.Write(resE)
		return
	}
	id, err := getValue(reqData, "id")
	if err != nil {
		resE := responseErrFunc(ParameterErr, jsonrpc, 0, errorMessage("getValue", err))
		w.Write(resE)
		return
	}

	switch method {
	case ETH_CHAINID:
		chainId := s.eth_chainId()
		resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: chainId})
		if err != nil {
			resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_chainId Marshal", err))
			w.Write(resE)
		} else {
			w.Write(resp)
		}
	case NET_VERSION:
		networkId := s.net_version()
		resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: networkId})
		if err != nil {
			resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("net_version Marshal", err))
			w.Write(resE)
		} else {
			w.Write(resp)
		}
	case ETH_CALL:
		ret, err := s.eth_call(reqData)
		if err != nil {
			var RetErr ErrorBody
			RetErr.Code = -4688
			RetErr.Message = err.Error()
			if len(ret) > 0 {
				btret := common.Hex2Bytes(ret)
				lenth := binary.BigEndian.Uint32(btret[64:68])
				data := btret[68 : lenth+68]
				errMsg := string(data)
				RetErr.Message = RetErr.Message + ": " + errMsg
				RetErr.Data = stringToHex(ret)
			}

			resp, err := json.Marshal(responseErr{JsonRPC: jsonrpc, Id: id, Error: &RetErr})
			if err != nil {
				resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_call Marshal 1", err))
				w.Write(resE)
			} else {
				w.Write(resp)
			}
		} else {
			resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: stringToHex(ret)})
			if err != nil {
				resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_call Marshal 2", err))
				w.Write(resE)
			} else {
				w.Write(resp)
			}
		}
	case ETH_BLOCKNUMBER:
		num := s.eth_blockNumber()
		if num == 0 {
			resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("", fmt.Errorf("get max block number error: %v", num)))
			w.Write(resE)
		} else {
			resNum := fmt.Sprintf("%X", num)
			resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: (stringToHex(resNum))})
			if err != nil {
				resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_blockNumber Marshal", err))
				w.Write(resE)
			} else {
				w.Write(resp)
			}
		}
	case ETH_GETBALANCE:
		para, err := getParam(reqData)
		if err != nil || len(para) == 0 {
			resE := responseErrFunc(ParameterErr, jsonrpc, id, errorMessage("ETH_GETBALANCE getParam", err))
			w.Write(resE)
		} else {
			from := para[0].(string)
			blc, err := s.eth_getBalance(from)
			if err != nil {
				resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("eth_getBalance 1", err))
				w.Write(resE)
			} else {
				//metamask's decimal is 18,kto is 11,we need do blc*Pow10(7).
				bigB := blockchain.Uint64ToBigInt(blc)
				bl := bigB.Mul(bigB, big.NewInt(ETHKTODIC))

				resBalance := fmt.Sprintf("%X", bl)

				resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: (stringToHex(resBalance))})
				if err != nil {
					resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_getBalance 2", err))
					w.Write(resE)
				} else {
					w.Write(resp)
				}
			}
		}
	case ETH_GETBLOCKBYHASH:
		para, err := getParam(reqData)
		if err != nil || len(para) == 0 {
			resE := responseErrFunc(ParameterErr, jsonrpc, id, errorMessage("ETH_GETBLOCKBYHASH getParam", err))
			w.Write(resE)
		} else {
			blk, err := s.eth_getBlockByHash(para[0].(string), para[1].(bool))
			if err != nil {
				resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("eth_getBlockByHash", err))
				w.Write(resE)
			} else {
				resp, err := json.Marshal(responseBlock{JsonRPC: jsonrpc, Id: id, Result: blk})
				if err != nil {
					resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_getBlockByHash Marshal", err))
					w.Write(resE)
				} else {
					w.Write(resp)
				}
			}
		}
	case ETH_GETBLOCKBYNUMBER:
		para, err := getParam(reqData)
		if err != nil || len(para) == 0 {
			resE := responseErrFunc(ParameterErr, jsonrpc, id, errorMessage("ETH_GETBLOCKBYNUMBER getParam", err))
			w.Write(resE)
		} else {
			strNum := para[0].(string)
			var num uint64
			if strNum == "latest" {
				n, err := s.cli.GetMaxBlockNumber()
				if err != nil {
					resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("GetMaxBlockNumber", err))
					w.Write(resE)
					break
				}
				num = n
			} else {
				n, err := hexToUint64(strNum)
				if err != nil {
					resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("ETH_GETBLOCKBYNUMBER hexToUint64", err))
					w.Write(resE)
					break
				}
				num = n
			}
			if num <= 0 {
				num = 1
			}
			blk, err := s.eth_getBlockByNumber(num, para[1].(bool))
			if err != nil {
				resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("eth_getBlockByNumber", err))
				w.Write(resE)
			} else {
				resp, err := json.Marshal(responseBlock{JsonRPC: jsonrpc, Id: id, Result: blk})
				if err != nil {
					resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_getBlockByNumber Marshal", err))
					w.Write(resE)
				} else {
					w.Write(resp)
				}
			}
		}
	case ETH_GETTRANSACTIONBYHASH:
		para, err := getParam(reqData)
		if err != nil || len(para) == 0 {
			resE := responseErrFunc(ParameterErr, jsonrpc, id, errorMessage("ETH_GETTRANSACTIONBYHASH getParam", err))
			w.Write(resE)
		} else {
			hash := para[0].(string)
			tx, err := s.eth_getTransactionByHash(hash)
			if err != nil {
				resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("eth_getTransactionByHash", err))
				w.Write(resE)
			} else {
				resp, err := json.Marshal(responseTransaction{JsonRPC: jsonrpc, Id: id, Result: tx})
				if err != nil {
					resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("ETH_GETTRANSACTIONBYHASH Marshal", err))
					w.Write(resE)
				} else {
					w.Write(resp)
				}
			}
		}
	case ETH_GASPRICE:
		resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: s.eth_gasPrice()})
		if err != nil {
			resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_gasPrice Marshal", err))
			w.Write(resE)
		} else {
			w.Write(resp)
		}
	case EHT_GETCODE:
		para, err := getParam(reqData)
		if err != nil || len(para) == 0 {
			resE := responseErrFunc(ParameterErr, jsonrpc, id, errorMessage("EHT_GETCODE getParam", err))
			w.Write(resE)
		} else {
			code := s.eth_getCode(para[0].(string))
			resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: code})
			if err != nil {
				w.Write([]byte(errorMessage("eth_getCode Marshal getParam", err)))
			} else {
				w.Write(resp)
			}
		}
	case ETH_GETTRANSACTIONCOUNT:
		para, err := getParam(reqData)
		if err != nil || len(para) == 0 {
			resE := responseErrFunc(ParameterErr, jsonrpc, id, errorMessage("ETH_GETTRANSACTIONCOUNT getParam", err))
			w.Write(resE)
		} else {
			addr := para[0].(string)
			count, err := s.eth_getTransactionCount(addr)
			if err != nil {
				resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("eth_getTransactionCount", err))
				w.Write(resE)
			} else {
				hexCount := fmt.Sprintf("%X", count)
				resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: (stringToHex(hexCount))})
				if err != nil {
					resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_getTransactionCount Marshal", err))
					w.Write(resE)
				} else {
					w.Write(resp)
				}
			}
		}
	case ETH_ESTIMATEGAS:
		ret, err := s.eth_estimateGas(reqData)
		if err != nil {
			var RetErr ErrorBody
			RetErr.Code = -4677
			RetErr.Message = err.Error()
			if len(ret) > 0 {
				btret := common.Hex2Bytes(ret)
				lenth := binary.BigEndian.Uint32(btret[64:68])
				data := btret[68 : lenth+68]
				errMsg := string(data)
				RetErr.Message = RetErr.Message + ": " + errMsg
				RetErr.Data = stringToHex(ret)
			}

			resp, err := json.Marshal(responseErr{JsonRPC: jsonrpc, Id: id, Error: &RetErr})
			if err != nil {
				resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_estimateGas Marshal 1", err))
				w.Write(resE)
			} else {
				w.Write(resp)
			}
		} else {
			resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: stringToHex(ret)})
			if err != nil {
				resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_estimateGas Marshal 2", err))
				w.Write(resE)
			} else {
				w.Write(resp)
			}
		}
	case ETH_SENDRAWTRANSACTION:
		para, err := getParam(reqData)
		if err != nil || len(para) == 0 {
			resE := responseErrFunc(ParameterErr, jsonrpc, id, errorMessage("ETH_SENDRAWTRANSACTION getParam", err))
			w.Write(resE)
		} else {
			hash, err := s.eth_sendRawTransaction(para[0].(string))
			if err != nil {
				resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("eth_sendRawTransaction", err))
				w.Write(resE)
			} else {
				resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: hash})
				if err != nil {
					resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_sendRawTransaction Marshal", err))
					w.Write(resE)
				} else {
					w.Write(resp)
				}
			}
		}
	case ETH_GETTRANSACTIONRECEIPT:
		para, err := getParam(reqData)
		if err != nil || len(para) == 0 {
			resE := responseErrFunc(ParameterErr, jsonrpc, id, errorMessage("ETH_GETTRANSACTIONRECEIPT getParam", err))
			w.Write(resE)
		} else {
			tc, err := s.eth_getTransactionReceipt(para[0].(string))
			if err != nil {
				resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("eth_getTransactionReceipt", err))
				w.Write(resE)
			} else {
				resp, err := json.Marshal(responseReceipt{JsonRPC: jsonrpc, Id: id, Result: tc})
				if err != nil {
					resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_getTransactionReceipt Marshal", err))
					w.Write(resE)
				} else {
					w.Write(resp)
				}
			}
		}

	case ETH_GETLOGS:
		res, err := s.eth_getLogs(reqData)
		if err != nil {
			resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("eth_getLogs", err))
			w.Write(resE)
		} else {
			resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: res})
			if err != nil {
				resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_getLogs Marshal", err))
				w.Write(resE)
			} else {
				w.Write(resp)
			}
		}
	case WEB3_CLIENTVERSION:
		res := s.web3_clientVersion()
		resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: res})
		if err != nil {
			resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("web3_clientVersion Marshal", err))
			w.Write(resE)
		} else {
			w.Write(resp)
		}

	case ETH_GETSTORAGEAT:
		res, err := s.eth_getStorageAt(reqData)
		if err != nil {
			resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("eth_getStorageAt", err))
			w.Write(resE)
		} else {
			resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: res})
			if err != nil {
				resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_getStorageAt Marshal", err))
				w.Write(resE)
			} else {
				w.Write(resp)
			}
		}
	case ETH_SIGNTRANSACTION:
		signatrue, err := s.eth_signTransaction(reqData)
		if err != nil {
			resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("eth_signTransaction", err))
			w.Write(resE)
		} else {
			resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: signatrue})
			if err != nil {
				resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_signTransaction Marshal", err))
				w.Write(resE)
			} else {
				w.Write(resp)
			}
		}

	case ETH_ACCOUNTS:
		accounts := s.eth_accounts()
		resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: accounts})
		if err != nil {
			resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("eth_accounts Marshal", err))
			w.Write(resE)
		} else {
			w.Write(resp)
		}

	case PERSONAL_UNLOCKACCOUNT:
		resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: true})
		if err != nil {
			resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("personal_unlockAccount Marshal", err))
			w.Write(resE)
		} else {
			w.Write(resp)
		}
	case NET_LISTENING:
		resp, err := json.Marshal(responseBody{JsonRPC: jsonrpc, Id: id, Result: true})
		if err != nil {
			resE := responseErrFunc(JsonMarshalErr, jsonrpc, id, errorMessage("net_listening Marshal", err))
			w.Write(resE)
		} else {
			w.Write(resp)
		}
	default:
		resE := responseErrFunc(UnkonwnErr, jsonrpc, id, errorMessage("", fmt.Errorf("Unsupport method:%v", method)))
		w.Write(resE)
	}
	return
}

//Returns the current chain id.
func (s *Server) eth_chainId() string {
	return s.Cfg.MetamaskCfg.ChainId
}

//Returns the current network id.
func (s *Server) net_version() string {
	return s.Cfg.MetamaskCfg.NetworkId
}

//Returns a list of addresses owned by client.
func (s *Server) eth_accounts() []common.Address {
	var accounts []common.Address
	return accounts
}

//Signs a transaction that can be submitted to the network at a later time using with eth_sendRawTransaction.
func (s *Server) eth_signTransaction(mp map[string]interface{}) (string, error) {
	return "", errors.New("unsupport method")
}

//send signed transaction
func (s *Server) eth_sendRawTransaction(rawTx string) (string, error) {
	if rawTx[:2] != "0x" {
		rawTx = "0x" + rawTx
	}
	return s.cli.SendRawTransaction(rawTx)
}

//Executes a new message call immediately without creating a transaction on the block chain.
func (s *Server) eth_call(mp map[string]interface{}) (string, error) {
	v, ok := mp["params"]
	if !ok {
		return "", errors.New(fmt.Sprintf("'%s' not exist", "params"))
	}

	if _, ok := v.([]interface{}); !ok {
		return "", errors.New("eth_call: params is wrong!")
	}

	Para := v.([]interface{})
	var para params

	if len(Para) > 0 {
		err := mapstructure.Decode(Para[0].(map[string]interface{}), &para)
		if err != nil {
			return "", err
		}
	}
	ret, _, err := s.cli.ContractCall(para.From, para.To, para.Data, blockchain.Check0x(para.Value))
	return ret, err
}

//Returns the number of most recent block.
func (s *Server) eth_blockNumber() uint64 {
	num, err := s.cli.GetMaxBlockNumber()
	if err != nil {
		return 0
	}
	return num
}

//Returns the balance of the account of given address.
func (s *Server) eth_getBalance(from string) (uint64, error) {
	return s.cli.GetBalance(from)
}

//Returns information about a block by block hash.
func (s *Server) eth_getBlockByHash(hash string, bl bool) (*Block, error) {
	b, err := s.cli.GetBlockByHash(hash)
	if err != nil {
		return nil, err
	}
	return s.ktoBlockToEthBlock(b, bl), nil
}

//Returns information about a block by block number.
func (s *Server) eth_getBlockByNumber(num uint64, bl bool) (*Block, error) {
	b, err := s.cli.GetBlockByNumber(num)
	if err != nil {
		return nil, err
	}
	return s.ktoBlockToEthBlock(b, bl), nil
}

//Returns the information about a transaction requested by transaction hash.
func (s *Server) eth_getTransactionByHash(hash string) (*Transaction, error) {
	tx, err := s.cli.GetTransactionByHash(hash)
	if err != nil {
		return nil, err
	}

	var trs Transaction
	from, _ := tx.From.NewCommonAddr()
	trs.From = from
	to, _ := tx.To.NewCommonAddr()
	trs.To = to

	trs.GasPrice = uint64ToHexString(tx.GasPrice)
	trs.Gas = uint64ToHexString(tx.GasUsed)
	trs.Hash = common.BytesToHash(tx.Hash())
	trs.Nonce = uint64ToHexString(tx.Nonce)
	trs.Value = uint64ToHexString(tx.Amount)

	if tx.BlockNum > 0 {
		b, err := s.cli.GetBlockByNumber(tx.BlockNum)
		if err != nil {
			return nil, err
		}

		trs.BlockHash = common.BytesToHash(b.Hash)
		trs.BlockNumber = uint64ToHexString(b.Height)

		for id, t := range b.Transactions {
			if t.HashToString() == tx.HashToString() {
				trs.TransactionIndex = uint64ToHexString(uint64(id))
				break
			}
		}
	}

	evm, _ := transaction.DecodeEvmData(tx.Input)
	if evm != nil {
		if evm.Operation == "create" || evm.Operation == "Create" {
			trs.To = common.Address{}
		}

		ethtx, err := transaction.DecodeEthData(evm.EthData)
		if err != nil {
			return nil, err
		}

		v, r, s := ethtx.RawSignatureValues()

		trs.R = common.BytesToHash(r.Bytes())
		trs.S = common.BytesToHash(s.Bytes())

		v = new(big.Int).Sub(v, new(big.Int).Mul(ethtx.ChainId(), big.NewInt(2)))
		trs.V = uint64ToHexString(v.Uint64())
	}
	return &trs, nil
}

//Returns code at a given address.
func (s *Server) eth_getCode(addr string) string {
	return s.cli.GetCode(addr)
}

//Returns the number of transactions sent from an address.
func (s *Server) eth_getTransactionCount(addr string) (uint64, error) {
	return s.cli.GetNonce(addr)
}

//Returns the current price per gas in wei.
func (s *Server) eth_gasPrice() string {
	return uint64ToHexString(s.Cfg.ChainCfg.GasLimit)
}

//Generates and returns an estimate of how much gas is necessary to allow the transaction to complete. The transaction will not be added to the blockchain.
//Note that the estimate may be significantly more than the amount of gas actually used by the transaction, for a variety of reasons including EVM mechanics and node performance.
func (s *Server) eth_estimateGas(mp map[string]interface{}) (string, error) {
	v, ok := mp["params"]
	if !ok {
		return "", errors.New(fmt.Sprintf("'%s' not exist", "params"))
	}

	if _, ok := v.([]interface{}); !ok {
		return "", errors.New("eth_estimateGas: params is wrong!")
	}

	Para := v.([]interface{})
	var para params

	if len(Para) > 0 {
		err := mapstructure.Decode(Para[0].(map[string]interface{}), &para)
		if err != nil {
			return "", err
		}
	}

	if len(para.Data) <= 0 {
		return fmt.Sprintf("%X", s.Cfg.ChainCfg.GasLimit), nil
	}

	if len(para.To) <= 0 {
		return fmt.Sprintf("%X", s.Cfg.ChainCfg.GasLimit), nil
	}

	ret, gas, err := s.cli.ContractCall(para.From, para.To, para.Data, blockchain.Check0x(para.Value))
	if err != nil {
		return ret, err
	}
	return gas, nil
}

//Returns the receipt of a transaction by transaction hash.
func (s *Server) eth_getTransactionReceipt(hash string) (*TransactionReceipt, error) {
	hash = blockchain.Check0x(hash)
	tx, err := s.cli.GetTransactionReceipt(hash)
	if err != nil {
		return nil, nil
	}
	b, err := s.cli.GetBlockByNumber(tx.BlockNum)
	if err != nil {
		return nil, nil
	}
	return s.ktoTxToTxReceipt(tx, b), nil
}

//Returns an array of all logs matching a given filter object.
func (s *Server) eth_getLogs(mp map[string]interface{}) ([]*types.Log, error) {
	v, ok := mp["params"]
	if !ok {
		return nil, errors.New(fmt.Sprintf("'%s' not exist", "params"))
	}

	if _, ok := v.([]interface{}); !ok {
		return nil, errors.New("eth_call: params is wrong!")
	}

	Para := v.([]interface{})
	var para reqGetLog

	if len(Para) > 0 {
		err := mapstructure.Decode(Para[0].(map[string]interface{}), &para)
		if err != nil {
			return nil, err
		}
	}

	var fromBlock, toBlock uint64
	if len(para.FromBlock) > 0 && len(para.ToBlock) > 0 {
		fb, err := hexToUint64(para.FromBlock)
		if err != nil {
			return nil, err
		}
		fromBlock = fb

		tb, err := hexToUint64(para.ToBlock)
		if err != nil {
			return nil, err
		}
		toBlock = tb
	}

	return s.cli.GetLogs(para.Address, fromBlock, toBlock, para.Topics, para.BlockHash), nil
}

//Returns the current client version.
func (s *Server) web3_clientVersion() string {
	return s.Cfg.MetamaskCfg.ClinetVersion
}

//Returns the value from a storage position at a given address.
func (s *Server) eth_getStorageAt(mp map[string]interface{}) (string, error) {
	v, ok := mp["params"]
	if !ok {
		return "", errors.New(fmt.Sprintf("'%s' not exist", "params"))
	}

	var addr, hash string
	if paras, ok := v.([]interface{}); ok {
		if len(paras) == 1 {
			addr = paras[0].(string)
		} else if len(paras) == 2 {
			addr = paras[0].(string)
			hash = paras[1].(string)
		}
	} else {
		return "", errors.New("eth_getStorageAt: params is wrong!")
	}
	return s.cli.GetStorageAt(addr, hash), nil
}

//kto block to eth block
func (s *Server) ktoBlockToEthBlock(b *block.Block, bl bool) *Block {
	var block Block
	block.Difficulty = stringToHex(b.Difficulty.Text(16))
	block.GasUsed = uint64ToHexString(b.GasUsed)
	block.GasLimit = uint64ToHexString(b.GasLimit)
	block.Nonce = uint64ToHexString(b.Nonce)

	block.Hash = common.BytesToHash(b.Hash)
	block.Number = uint64ToHexString(b.Height)
	block.ParentHash = common.BytesToHash(b.PrevHash)
	block.TimeStamp = uint64ToHexString(b.Timestamp)

	miner, _ := b.Miner.NewCommonAddr()
	block.Miner = miner

	if bl {
		block.Transactions = s.ktoTxsToEthTxs(b, b.Transactions)
	} else {
		block.Transactions = s.getTxsHashes(b.Transactions)
	}
	return &block
}

//kto txs to eth txs
func (s *Server) ktoTxsToEthTxs(block *block.Block, ktxs []*transaction.FinishedTransaction) []*TransactionReceipt {
	var etxs []*TransactionReceipt
	for _, ktx := range ktxs {
		etxs = append(etxs, s.ktoTxToTxReceipt(ktx, block))
	}
	return etxs
}

//get txs hashes from transactions
func (s *Server) getTxsHashes(ktxs []*transaction.FinishedTransaction) []common.Hash {
	var hashes []common.Hash
	for _, ktx := range ktxs {
		hashes = append(hashes, common.BytesToHash(ktx.Hash()))
	}
	return hashes
}

//kto tx to eth tx receipt
func (s *Server) ktoTxToTxReceipt(tx *transaction.FinishedTransaction, block *block.Block) *TransactionReceipt {
	var trp TransactionReceipt
	var blockHash []byte
	trp.TransactionHash = common.BytesToHash(tx.Hash())
	trp.BlockNumber = uint64ToHexString(tx.BlockNum)
	trp.GasUsed = uint64ToHexString(tx.GasUsed)
	trp.CumulativeGasUsed = trp.GasUsed

	from, _ := tx.From.NewCommonAddr()
	trp.From = from
	to, _ := tx.To.NewCommonAddr()
	trp.To = to

	var index uint
	if block != nil {
		for id, t := range block.Transactions {
			if t.HashToString() == tx.HashToString() {
				trp.TransactionIndex = uint64ToHexString(uint64(id))
				index = uint(id)
				break
			}
		}
		blockHash = block.Hash
	}

	trp.BlockHash = common.BytesToHash(blockHash)

	evm, _ := transaction.DecodeEvmData(tx.Input)
	if evm != nil {
		trp.ContractAddress = evm.ContractAddr
		if evm.Operation == blockchain.CREATECONTRACT {
			trp.To = common.Address{}
		} else {
			trp.To = evm.ContractAddr
		}

		var stat uint64
		if !evm.Status {
			stat = 0
			trp.Status = FAILED
		} else {
			stat = 1
			trp.Status = SUCCESS
		}
		trp.Logs = evm.Logs
		if len(trp.Logs) == 0 {
			trp.Logs = make([]*types.Log, 0)
		}

		//set bloom
		receipt := &types.Receipt{
			Type:              uint8(tx.Type),
			Status:            stat,
			Logs:              evm.Logs,
			TxHash:            common.BytesToHash(tx.Hash()),
			ContractAddress:   trp.ContractAddress,
			GasUsed:           tx.GasUsed,
			BlockHash:         common.BytesToHash(blockHash),
			TransactionIndex:  index,
			BlockNumber:       blockchain.Uint64ToBigInt(tx.BlockNum),
			PostState:         trp.Root.Bytes(),
			CumulativeGasUsed: tx.GasUsed,
		}
		receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

		trp.LogsBloom = receipt.Bloom
	} else {
		trp.Status = FAILED
	}
	return &trp
}
