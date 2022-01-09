package grpcserver

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"

	"github.com/ethereum/go-ethereum/common"
	evmtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/blockchain"
	"github.com/korthochain/korthochain/pkg/config"
	"github.com/korthochain/korthochain/pkg/crypto"
	_ "github.com/korthochain/korthochain/pkg/crypto/sigs/ed25519"
	_ "github.com/korthochain/korthochain/pkg/crypto/sigs/secp"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/miner"
	"github.com/korthochain/korthochain/pkg/p2p"
	"github.com/korthochain/korthochain/pkg/server"
	"github.com/korthochain/korthochain/pkg/server/grpcserver/message"

	"github.com/korthochain/korthochain/pkg/transaction"
	"github.com/korthochain/korthochain/pkg/txpool"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Greeter struct {
	Bc       blockchain.Blockchains
	Tp       *txpool.Pool
	Cfg      *config.CfgInfo
	Node     *p2p.Node
	Miner    *miner.Miner
	NodeName string
}

func NewGreeter(bc blockchain.Blockchains, tp *txpool.Pool, cfg *config.CfgInfo) *Greeter {
	return &Greeter{Bc: bc, Tp: tp, Cfg: cfg}
}

func (g *Greeter) RunGrpc() {
	lis, err := net.Listen("tcp", g.Cfg.SververCfg.GRpcAddress)
	if err != nil {
		logger.Error("net.Listen", zap.Error(err))
		os.Exit(-1)
	}

	server := grpc.NewServer(grpc.UnaryInterceptor(server.IpInterceptor))

	message.RegisterGreeterServer(server, g)

	if err := server.Serve(lis); err != nil {
		panic(err)
	}
}

//get balance
func (g *Greeter) GetBalance(ctx context.Context, in *message.ReqBalance) (*message.ResBalance, error) {
	addr, err := address.StringToAddress(in.Address)
	if err != nil {
		logger.Info("StringToAddress error", zap.Error(err))
		return nil, fmt.Errorf("eth addr to kto addr error:%s", err.Error())
	}
	balance, err := g.Bc.GetBalance(addr)
	if err != nil {
		logger.Error("g.Bc.GetBalance", zap.Error(err), zap.String("address", in.Address))
		return nil, err
	}
	return &message.ResBalance{Balance: balance}, nil
}

//send transaction to chain
func (g *Greeter) SendTransaction(ctx context.Context, in *message.ReqTransaction) (*message.ResTransaction, error) {

	from, err := address.NewAddrFromString(in.From)
	if err != nil {
		return nil, err
	}

	to, err := address.NewAddrFromString(in.To)
	if err != nil {
		return nil, err
	}

	balance, err := g.Bc.GetAvailableBalance(from)
	if err != nil {
		return nil, err
	}
	if in.Amount+in.GasLimit*in.GasPrice > balance {
		return nil, fmt.Errorf("from(%v) balance(%v) is not enough or out of gas.", from, balance)
	}
	sign, err := crypto.DeserializeSignature(in.Sign)
	if err != nil {
		return nil, err
	}

	if in.GasLimit*in.GasPrice == 0 {
		return nil, fmt.Errorf("error: one of gasprice[%v] and gaslimit[%v] is 0", in.GasLimit, in.GasPrice)
	}

	tx := &transaction.SignedTransaction{
		Transaction: transaction.Transaction{
			From:      from,
			To:        to,
			Amount:    in.Amount,
			Nonce:     in.Nonce,
			GasLimit:  in.GasLimit,
			GasPrice:  in.GasPrice,
			GasFeeCap: in.GasFeeCap,
			Input:     in.Input,
			Type:      transaction.TransferTransaction,
		},
		Signature: *sign,
	}

	err = g.Tp.Add(tx)
	if err != nil {
		return nil, err
	}

	data, err := tx.Serialize()
	if err != nil {
		return nil, err
	}

	if g.Node != nil {
		g.Node.SendMessage(p2p.PayloadMessageType, append([]byte{0}, data...))
	}

	hash := hex.EncodeToString(tx.Hash())

	return &message.ResTransaction{Hash: hash}, nil
}

//send Lock transaction to chain
func (g *Greeter) SendLockTransaction(ctx context.Context, in *message.ReqTransaction) (*message.ResTransaction, error) {

	from, err := address.NewAddrFromString(in.From)
	if err != nil {
		return nil, err
	}

	to, err := address.NewAddrFromString(in.To)
	if err != nil {
		return nil, err
	}
	balance, err := g.Bc.GetAvailableBalance(from)
	if err != nil {
		return nil, err
	}
	if in.Amount+in.GasLimit*in.GasPrice > balance {
		return nil, errors.New("freeze balance is not enough or out of gas.")
	}

	sign, err := crypto.DeserializeSignature(in.Sign)
	if err != nil {
		return nil, err
	}

	if in.GasLimit*in.GasPrice == 0 {
		return nil, fmt.Errorf("error: one of gasprice[%v] and gaslimit[%v] is 0", in.GasLimit, in.GasPrice)
	}

	tx := &transaction.SignedTransaction{
		Transaction: transaction.Transaction{
			From:      from,
			To:        to,
			Amount:    in.Amount,
			Nonce:     in.Nonce,
			GasLimit:  in.GasLimit,
			GasPrice:  in.GasPrice,
			GasFeeCap: in.GasFeeCap,
			Input:     in.Input,
			Type:      transaction.LockTransaction,
		},
		Signature: *sign,
	}

	err = g.Tp.Add(tx)
	if err != nil {
		return nil, err
	}
	hash := hex.EncodeToString(tx.Hash())

	return &message.ResTransaction{Hash: hash}, nil
}

//send Unlock transaction to chain
func (g *Greeter) SendUnlockTransaction(ctx context.Context, in *message.ReqTransaction) (*message.ResTransaction, error) {
	from, err := address.NewAddrFromString(in.From)
	if err != nil {
		return nil, err
	}

	to, err := address.NewAddrFromString(in.To)
	if err != nil {
		return nil, err
	}

	balance, err := g.Bc.GetAllFreezeBalance(from)
	if err != nil {
		return nil, err
	}

	available, err := g.Bc.GetAvailableBalance(from)
	if err != nil {
		return nil, err
	}

	FreezeBalance, err := g.Bc.GetSingleFreezeBalance(from, to)
	if err != nil {
		return nil, err
	}

	if in.GasLimit*in.GasPrice > available || in.Amount > balance || in.Amount > FreezeBalance {
		return nil, errors.New("freeze balance is not enough or out of gas.")
	}

	sign, err := crypto.DeserializeSignature(in.Sign)
	if err != nil {
		return nil, err
	}

	if in.GasLimit*in.GasPrice == 0 {
		return nil, fmt.Errorf("error: one of gasprice[%v] and gaslimit[%v] is 0", in.GasLimit, in.GasPrice)
	}

	tx := &transaction.SignedTransaction{
		Transaction: transaction.Transaction{
			From:      from,
			To:        to,
			Amount:    in.Amount,
			Nonce:     in.Nonce,
			GasLimit:  in.GasLimit,
			GasPrice:  in.GasPrice,
			GasFeeCap: in.GasFeeCap,
			Input:     in.Input,
			Type:      transaction.UnlockTransaction,
		},
		Signature: *sign,
	}

	err = g.Tp.Add(tx)
	if err != nil {
		return nil, err
	}
	hash := hex.EncodeToString(tx.Hash())

	return &message.ResTransaction{Hash: hash}, nil
}

//Get Block By block Number
func (g *Greeter) GetBlockByNum(ctx context.Context, in *message.ReqBlockByNumber) (*message.RespBlock, error) {
	b, err := g.Bc.GetBlockByHeight(in.Height)
	if err != nil {
		logger.Error("GetBlockByHeight", zap.Error(fmt.Errorf("error:%v,height %v", err.Error(), in.Height)))
		return &message.RespBlock{Data: []byte{}, Code: -1, Message: err.Error()}, nil
	}
	blockbyte, err := b.Serialize()
	if err != nil {
		logger.Error("Serialize", zap.String("error", err.Error()))
		return &message.RespBlock{Data: []byte{}, Code: -1, Message: err.Error()}, nil
	}
	return &message.RespBlock{Data: blockbyte, Code: 0}, nil

}

//Get Block By block Hash
func (g *Greeter) GetBlockByHash(ctx context.Context, in *message.ReqBlockByHash) (*message.RespBlockDate, error) {
	hash, err := transaction.StringToHash(blockchain.Check0x(in.Hash))
	if err != nil {
		logger.Error("StringToHash", zap.String("error", err.Error()), zap.String("hash", in.Hash))
		return &message.RespBlockDate{Data: []byte{}, Code: -1, Message: err.Error()}, nil
	}
	block, err := g.Bc.GetBlockByHash(hash)
	if err != nil {
		//logger.Error("GetBlockByHash", zap.Error(fmt.Errorf("error:%v,hash: %v", err.Error(), in.Hash)))
		return &message.RespBlockDate{Data: []byte{}, Code: -1, Message: err.Error()}, nil
	}
	blockbyte, err := block.Serialize()
	if err != nil {
		logger.Error("Serialize", zap.String("error", err.Error()))
		return &message.RespBlockDate{Data: []byte{}, Code: -1, Message: err.Error()}, nil
	}
	return &message.RespBlockDate{Data: blockbyte, Code: 0}, nil
}

//Get transaction By Hash
func (g *Greeter) GetTxByHash(ctx context.Context, in *message.ReqTxByHash) (*message.RespTxByHash, error) {
	hash, _ := transaction.StringToHash(blockchain.Check0x(in.Hash))
	tx, err := g.Bc.GetTransactionByHash(hash)
	if err != nil {
		//logger.Error("GetTransactionByHash", zap.Error(fmt.Errorf("error:%v,hash:%v", err.Error(), in.Hash)))
		st, err := g.Tp.GetTxByHash(in.Hash)
		if err != nil {
			return &message.RespTxByHash{Data: []byte{}, Code: -1, Message: err.Error()}, nil
		}
		tx = &transaction.FinishedTransaction{SignedTransaction: *st}
	}

	txbytes, err := tx.Serialize()
	if err != nil {
		logger.Error("tx Serialize", zap.String("error", err.Error()))
		return &message.RespTxByHash{Data: []byte{}, Code: -1, Message: err.Error()}, nil
	}

	return &message.RespTxByHash{Data: txbytes, Code: 0}, nil
}

//Get Address Nonce
func (g *Greeter) GetAddressNonceAt(ctx context.Context, in *message.ReqNonce) (*message.ResposeNonce, error) {
	addr, err := address.StringToAddress(in.Address)
	if err != nil {
		logger.Info("StringToAddress error", zap.Error(err))
		return nil, fmt.Errorf("eth addr to kto addr error:%s", err.Error())
	}
	resp, err := g.Bc.GetNonce(addr)
	if err != nil {
		return nil, err
	}

	return &message.ResposeNonce{Nonce: resp}, nil
}

//send eth signed transaction
func (g *Greeter) SendEthSignedRawTransaction(ctx context.Context, in *message.ReqEthSignTransaction) (*message.ResEthSignTransaction, error) {
	ethFrom := common.HexToAddress(in.EthFrom)
	ethData := in.EthData

	l := len(ethData)
	if l <= 0 {
		logger.Error("Wrong eth signed data", zap.Error(fmt.Errorf("ethData length[%v] <= 0", len(ethData))))
		return nil, fmt.Errorf("Wrong eth signed data:%s", fmt.Errorf("ethData length[%v] <= 0", len(ethData)).Error())
	}
	logger.Info("rpc Into SendEthSignedRawTransaction:", zap.String("from", in.EthFrom), zap.Int32("data lenght", int32(l)))
	var from, to address.Address
	var evm transaction.EvmContract
	var amount, gasPrice, gasLimit uint64
	var tag transaction.TransactionType
	var signType crypto.SigType

	if len(in.KtoFrom) > 0 && len(in.MsgHash) > 0 {
		f, err := address.NewAddrFromString(in.KtoFrom)
		if err != nil {
			logger.Error("NewAddrFromString error", zap.Error(err))
			return nil, err
		}
		from = f
		signType = crypto.TypeED25519
		evm.MsgHash = in.MsgHash
	} else {
		addr, err := address.StringToAddress(in.EthFrom)
		if err != nil {
			logger.Info("StringToAddress error", zap.Error(err))
			return nil, fmt.Errorf("eth addr to kto addr error:%s", err.Error())
		}
		from = addr
		signType = crypto.TypeSecp256k1
	}

	ethTx, err := transaction.DecodeEthData(ethData)
	if err != nil {
		logger.Info("SendEthSignedTransaction decodeData error", zap.Error(err))
		return nil, fmt.Errorf("decodeData error:%s", err.Error())
	}
	if ethTx.ChainId().Int64() != g.Cfg.ChainCfg.ChainId {
		return nil, fmt.Errorf("error chain ID: %v", ethTx.ChainId().Int64())
	}

	evm.EthData = ethData

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
			logger.Info("StringToAddress error", zap.Error(err))
			return nil, fmt.Errorf("eth addr to kto addr error:%s", err.Error())
		}
		to = addr

		deci := new(big.Int).SetUint64(blockchain.ETHDECIMAL)
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
		logger.Info("EncodeEvmData error", zap.Error(err))
		return nil, fmt.Errorf("EncodeEvmData error:%s", err.Error())
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
		logger.Info("EthSignedRawTransaction add tx pool error:", zap.Error(err))
		return nil, fmt.Errorf(err.Error())
	}
	//g.n.Broadcast(tx)

	logger.Info("rpc End SendEthSignedRawTransaction:", zap.String("hash", tx.HashToString()))
	return &message.ResEthSignTransaction{Hash: tx.HashToString()}, nil
}

//call contract
func (g *Greeter) CallSmartContract(ctx context.Context, in *message.ReqCallContract) (*message.ResCallContract, error) {
	res, gas, err := g.Bc.CallSmartContract(in.Contractaddress, in.Origin, in.Inputcode, in.Value)
	if err != nil {
		logger.Error("rpc CallSmartContract", zap.String("Result", res), zap.String("error", err.Error()))
		return &message.ResCallContract{Result: res, Msg: err.Error(), Code: -1}, nil
	}
	return &message.ResCallContract{Result: res, Gas: gas, Code: 0}, nil
}

//get code by contract address
func (g *Greeter) GetCode(ctx context.Context, in *message.ReqEvmGetcode) (*message.ResEvmGetcode, error) {
	code := g.Bc.GetCode(in.Contract)
	if len(code) <= 0 {
		return nil, fmt.Errorf("code not exist")
	}
	return &message.ResEvmGetcode{Code: common.Bytes2Hex(code)}, nil
}

//get storage by hash
func (g *Greeter) GetStorageAt(ctx context.Context, in *message.ReqGetstorage) (*message.ResGetstorage, error) {
	ret := g.Bc.GetStorageAt(in.Addr, blockchain.Check0x(in.Hash))
	return &message.ResGetstorage{Result: ret.String()}, nil
}

//get max block height
func (g *Greeter) GetMaxBlockHeight(ctx context.Context, in *message.ReqMaxBlockHeight) (*message.ResMaxBlockHeight, error) {
	maxH, err := g.Bc.GetMaxBlockHeight()
	if err != nil {
		logger.Error("rpc GetMaxBlockHeight", zap.String("error", err.Error()))
		return nil, err
	}
	return &message.ResMaxBlockHeight{MaxHeight: maxH}, nil
}

//get evm logs
func (g *Greeter) GetLogs(ctx context.Context, in *message.ReqLogs) (*message.ResLogs, error) {
	if in.FromBlock > in.ToBlock {
		return nil, fmt.Errorf("Wrong fromBlock[%v] and to toBlock[%v]", in.FromBlock, in.ToBlock)
	}
	logs := g.Bc.GetLogs()
	var resLogs []*evmtypes.Log
	for _, log := range logs {
		if common.HexToHash(in.BlockHash) == log.BlockHash {
			resLogs = append(resLogs, log)
		} else if log.BlockNumber >= in.FromBlock && log.BlockNumber <= in.ToBlock {
			if common.HexToAddress(in.Address) == log.Address {
				resLogs = append(resLogs, log)
			}
		}
	}
	bslog, err := json.Marshal(&resLogs)
	if err != nil {
		return nil, err
	}
	return &message.ResLogs{Evmlogs: bslog}, nil
}

//Get Single Freeze Balance
func (g *Greeter) GetSingleFreezeBalance(ctx context.Context, in *message.ReqSignBalance) (*message.ResBalance, error) {

	from, err := address.NewAddrFromString(in.From)
	if err != nil {
		return nil, err
	}
	to, err := address.NewAddrFromString(in.To)
	if err != nil {
		return nil, err
	}

	bal, err := g.Bc.GetSingleFreezeBalance(from, to)
	if err != nil {
		return nil, err
	}
	return &message.ResBalance{Balance: bal}, nil
}

//Get All Freeze Balance
func (g *Greeter) GetAllFreezeBalance(ctx context.Context, in *message.ReqBalance) (*message.ResBalance, error) {
	from, err := address.NewAddrFromString(in.Address)
	if err != nil {
		return nil, err
	}

	bal, err := g.Bc.GetAllFreezeBalance(from)
	if err != nil {
		return nil, err
	}
	return &message.ResBalance{Balance: bal}, nil
}

//Get Available Balance
func (g *Greeter) GetAvailableBalance(ctx context.Context, in *message.ReqGetAvailableBalance) (*message.ResGetAvailableBalance, error) {
	addr, err := address.StringToAddress(in.Address)
	if err != nil {
		logger.Info("StringToAddress error", zap.Error(err))
		return nil, err
	}
	avi, err := g.Bc.GetAvailableBalance(addr)
	if err != nil {
		return nil, err
	}
	return &message.ResGetAvailableBalance{AvailableBalance: avi}, nil
}

//Get Hasher Per Second
func (g *Greeter) GetHasherPerSecond(ctx context.Context, req *message.ReqHasherPerSecond) (*message.ResHasherPerSecond, error) {
	resp := &message.ResHasherPerSecond{}
	if g.Miner == nil {
		resp.HasherPerSecond = 0
		return resp, nil
	}
	h := g.Miner.HashesPerSecond()
	resp.Address = h.MinerAddr.String()
	resp.HasherPerSecond = float32(h.HasherPerSecond)
	resp.Uuid = g.NodeName
	return resp, nil
}
