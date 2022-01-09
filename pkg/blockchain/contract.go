package blockchain

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/contract/evm"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/storage/store"

	"github.com/korthochain/korthochain/pkg/address"

	"github.com/ethereum/go-ethereum/common"
	evmtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/korthochain/korthochain/pkg/transaction"
	"go.uber.org/zap"
)

//binding kto address and eth address
func (bc *Blockchain) bindingAddress(DBTransaction store.Transaction, ethaddr, ktoaddr []byte) error {
	return DBTransaction.Mset(BindingKey, ethaddr, ktoaddr)
}

//get bingding kto address by eth address
func (bc *Blockchain) GetBindingKtoAddress(ethAddr string) (address.Address, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	DBTransaction := bc.db.NewTransaction()
	defer DBTransaction.Cancel()
	return getBindingKtoAddress(DBTransaction, ethAddr)
}

func getBindingKtoAddress(DBTransaction store.Transaction, ethAddr string) (address.Address, error) {
	data, err := DBTransaction.Mget(BindingKey, common.HexToAddress(ethAddr).Bytes())
	if err != nil {
		if err.Error() != "NotExist" {
			logger.Error("GetBindingKtoAddress error", zap.String("ethaddr", ethAddr), zap.Error(err))
		}
		return address.Address{}, err
	}
	return address.NewFromBytes(data)
}

//get bingding eth address by kto address
func (bc *Blockchain) GetBindingEthAddress(ktoAddr address.Address) (string, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	DBTransaction := bc.db.NewTransaction()
	defer DBTransaction.Cancel()

	ethAddr, err := DBTransaction.Mget(BindingKey, ktoAddr.Bytes())
	if err != nil {
		return "", err
	}

	return common.BytesToAddress(ethAddr).String(), nil
}

//call contract with out transaction
func (bc *Blockchain) CallSmartContract(contractAddr, origin, callInput, value string) (string, string, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	bc.evm = evm.NewEvm(bc.sdb, bc.ChainCfg.ChainId, bc.ChainCfg.GasLimit, bc.ChainCfg.GasPrice)

	snapshotId := bc.evm.GetSnapshot()
	defer func() {
		bc.evm.RevertToSnapshot(snapshotId)
	}()

	{
		//set block into into evm
		maxH, err := bc.getMaxBlockHeight()
		if err != nil {
			logger.Error("Failed to getMaxBlockHeight", zap.Error(err))
			return "", "", err
		}
		if maxH > 0 {
			currB, err := bc.getBlockByHeight(maxH)
			if err != nil {
				logger.Error("Failed to getBlockByHeight", zap.Error(err))
				return "", "", err
			}
			miner, _ := currB.Miner.NewCommonAddr()
			bc.evm.SetBlockInfo(currB.Height, currB.Timestamp, miner, currB.Difficulty)
		}
	}

	vl := big.NewInt(0)
	if len(value) > 0 {
		if res, ok := big.NewInt(0).SetString(value, 16); ok {
			vl = res.Div(res, Uint64ToBigInt(ETHDECIMAL))
		}
	}

	bc.evm.SetConfig(vl, Uint64ToBigInt(bc.ChainCfg.GasPrice), MAXGASLIMIT, common.HexToAddress(origin))

	if len(Check0x(callInput)) > 0 && len(contractAddr) > 0 {
		ret, gasleft, err := bc.evm.Call(common.HexToAddress(contractAddr), common.HexToAddress(origin), common.Hex2Bytes(Check0x(callInput)))
		if err != nil {
			logger.Info("call contract", zap.String("ret", common.Bytes2Hex(ret)), zap.Error(err))
			return common.Bytes2Hex(ret), "", err
		}
		gasUsed := MAXGASLIMIT - gasleft
		if gasUsed < MINGASLIMIT {
			gasUsed = MINGASLIMIT
		}
		return common.Bytes2Hex(ret), fmt.Sprintf("%X", gasUsed), nil
	}

	logger.Error("failed to Call", zap.Error(fmt.Errorf("wrong input[%v] or contractaddr[%v]", callInput, contractAddr)))
	return "", "", fmt.Errorf("wrong input[%v] or contractaddr[%v]", callInput, contractAddr)
}

//get contract code by address
func (bc *Blockchain) GetCode(contractAddr string) []byte {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.sdb.GetCode(common.HexToAddress(contractAddr))
}

//get contract code by address
func (bc *Blockchain) SetCode(contractAddr common.Address, code []byte) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	bc.sdb.SetCode(contractAddr, code)
	return
}

//get evm logs
func (bc *Blockchain) GetLogs() []*evmtypes.Log {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.evm.Logs()
}

//handle contract transaction
func (bc *Blockchain) handleContractTransaction(block *block.Block, DBTransaction store.Transaction, tx *transaction.FinishedTransaction, index int) (uint64, error) {
	var gasLeft, gasLimit uint64
	gasLimit = tx.Transaction.GasLimit * tx.Transaction.GasPrice
	if gasLimit == 0 {
		gasLimit = MAXGASLIMIT
	}
	evmC, err := transaction.DecodeEvmData(tx.Input)
	if err != nil {
		logger.Error("DecodeEvmData input error:", zap.Error(err))
		return gasLeft, err
	}

	eth_tx, err := transaction.DecodeEthData(evmC.EthData)
	if err != nil {
		logger.Error("DecodeEvmData input error:", zap.Error(err))
		return gasLeft, err
	}

	logger.Info("handleContractTransaction info", zap.Uint64("eth_tx gasprice", eth_tx.GasPrice().Uint64()), zap.Uint64("gaslimit", gasLimit), zap.Uint64("tx.gasprice", tx.GasPrice),
		zap.String("origin", evmC.Origin.Hex()), zap.Uint64("nonce", tx.Transaction.Nonce), zap.Uint64("eth_tx.Value()", eth_tx.Value().Uint64()))

	TxValue := eth_tx.Value().Div(eth_tx.Value(), Uint64ToBigInt(ETHDECIMAL))
	bc.evm.SetConfig(TxValue, Uint64ToBigInt(tx.GasPrice), gasLimit, evmC.Origin)
	bc.evm.Prepare(common.BytesToHash(tx.Hash()), common.BytesToHash(block.Hash), index)

	switch evmC.Operation {
	case "create", "Create":
		bc.evm.SetNonce(evmC.Origin, tx.Transaction.Nonce)
		ret, contractAddr, left, err := bc.evm.Create(evmC.CreateCode, evmC.Origin)

		gasLeft = left
		if gasLeft < MINGASLIMIT {
			gasLeft = MINGASLIMIT
		}

		if err != nil {
			logger.Error("faile to create contract", zap.Error(fmt.Errorf("hash:%v,gasLeft:%v,error:%v", hex.EncodeToString(tx.Hash()), gasLeft, err))) //zap.String("name", evmC.ContractName), zap.Uint64("gasLeft", gasLeft), zap.Error(callErr))
			evmC.Ret = common.Bytes2Hex(ret)
			evmC.Status = false
			return gasLeft, nil
		}
		evmC.ContractAddr = contractAddr
		logger.Info("Create contract successfully", zap.String("contract address:", evmC.ContractAddr.Hex()), zap.Uint64("gasLeft", gasLeft), zap.String("ret:", evmC.Ret), zap.String("origin", evmC.Origin.Hex()))
	case "call", "Call":
		bc.evm.SetNonce(evmC.Origin, tx.Transaction.Nonce+1)
		ret, left, err := bc.evm.Call(evmC.ContractAddr, evmC.Origin, evmC.CallInput)

		gasLeft = left
		if gasLeft < MINGASLIMIT {
			gasLeft = MINGASLIMIT
		}

		if err != nil {
			logger.Error("faile to Call", zap.String("hash", hex.EncodeToString(tx.Hash())), zap.Uint64("gasLeft", gasLeft), zap.Error(err))
			evmC.Ret = common.Bytes2Hex(ret)
			evmC.Status = false
			return gasLeft, nil
		}
		evmC.Ret = common.Bytes2Hex(ret)
		logger.Info("Call contract successfully", zap.String("contract address:", evmC.ContractAddr.Hex()), zap.Uint64("gasLeft", gasLeft), zap.String("ret:", evmC.Ret), zap.String("origin", evmC.Origin.Hex()))
	}
	evmC.Status = true
	evmC.Logs = bc.evm.GetLogs(common.BytesToHash(tx.Hash()), common.BytesToHash(block.Hash))

	input, err := transaction.EncodeEvmData(evmC)
	if err != nil {
		logger.Error("EncodeEvmData error", zap.Error(err))
		return gasLeft, nil

	}
	tx.Input = input
	return gasLeft, nil
}

//get storage at address
func (bc *Blockchain) GetStorageAt(addr, hash string) common.Hash {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.evm.GetStorageAt(common.HexToAddress(addr), common.HexToHash(hash))
}
