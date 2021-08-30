package blockchain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/config"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/storage/miscellaneous"
	"github.com/korthochain/korthochain/pkg/storage/store"

	"github.com/korthochain/korthochain/pkg/address"

	"github.com/ethereum/go-ethereum/common"
	evmtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/korthochain/korthochain/pkg/transaction"
	"go.uber.org/zap"
)

var ETHDECIMAL uint64 = 10000000

var (
	BindingKey = []byte("binding")
)

//binding kto address and eth address
func (bc *Blockchain) bindingAddress(DBTransaction store.Transaction, ethaddr, ktoaddr []byte) error {
	DBTransaction.Mdel(BindingKey, ethaddr)
	return DBTransaction.Mset(BindingKey, ethaddr, ktoaddr)
}

//get bingding kto address by eth address
func (bc *Blockchain) GetBindingKtoAddress(ethAddr string) (address.Address, error) {
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
	DBTransaction := bc.db.NewTransaction()
	defer DBTransaction.Cancel()

	ethAddr, err := DBTransaction.Mget(BindingKey, ktoAddr.Bytes())
	if err != nil {
		return "", err
	}

	return common.BytesToAddress(ethAddr).String(), nil
}

//call contract with out transaction
func (bc *Blockchain) CallSmartContract(contractAddr, origin, callInput, value string) (string, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if len(value) > 0 {
		if res, ok := big.NewInt(0).SetString(value, 16); ok {
			deci := new(big.Int).SetUint64(ETHDECIMAL)
			vl := res.Div(res, deci)
			bc.evm.SetConfig(vl, new(big.Int).SetUint64(21000), config.GlobalCfg.BFTConfig.GasLimit)
		}
	} else {
		bc.evm.SetConfig(big.NewInt(0).SetUint64(0), config.GlobalCfg.BFTConfig.GasLimit)
	}
	if len(callInput) > 2 {
		if callInput[:2] == "0x" {
			callInput = callInput[2:]
		}
	} else {
		return "", fmt.Errorf("wrong call input[%v]", callInput)
	}

	if len(callInput) > 0 && len(contractAddr) > 0 {
		snapshotId := bc.evm.GetSnapshot()

		dump1 := bc.evm.RawDump()
		ethAddrs := make(map[common.Address]*big.Int)
		//this for sets balance to the corresponding eth address [ethAddrs] in contract.
		for addr, _ := range dump1.Accounts {
			kAddr, err := bc.GetBindingKtoAddress(addr.String())
			if err != nil {
				continue
			}
			ka := miscellaneous.BytesSha1Address(kAddr.Bytes())
			kBalance := bc.sdb.GetBalance(ka)
			//fmt.Println("check set balance1", addr, kBalance)
			bc.sdb.SetBalance(addr, kBalance)
		}

		defer func() {
			//this for cleans Previously set[ethAddrs]
			bc.evm.RevertToSnapshot(snapshotId)

			for addr, _ := range ethAddrs {
				delete(ethAddrs, addr)
			}
		}()

		ret, gasLeft, err := bc.evm.Call(common.HexToAddress(contractAddr), common.HexToAddress(origin), common.Hex2Bytes(callInput))
		if err != nil {
			logger.Info("Call error", zap.String("Call error", err.Error()))
		} else {
			logger.Info("Call Ok", zap.String("contract addr", contractAddr), zap.String("ret:", common.Bytes2Hex(ret)), zap.Uint64("gasLeft", gasLeft))
		}
		rcfg := bc.evm.GetConfig()
		_, errs := bc.chargeGas(address.EthAddressToKtoAddress(common.HexToAddress(origin)), rcfg.GasLimit, gasLeft)
		if errs != nil {
			return common.Bytes2Hex(ret), errs
		}
		//set gasUsed to miner
		return common.Bytes2Hex(ret), err
	}

	logger.Error("failed to Call", zap.Error(fmt.Errorf("wrong input[%v] or contractaddr[%v]", callInput, contractAddr)))
	return "", fmt.Errorf("wrong input[%v] or contractaddr[%v]", callInput, contractAddr)
}

//get contract code by address
func (bc *Blockchain) GetCode(contractAddr string) []byte {
	code := bc.evm.GetCode(common.HexToAddress(contractAddr))
	return code
}

//get logs by hash
func (bc *Blockchain) GetLogs(hash common.Hash) []*evmtypes.Log {
	return bc.evm.GetLogs(hash)
}

//handle contract transaction
func (bc *Blockchain) handleContractTransaction(block *block.Block, DBTransaction store.Transaction, tx *transaction.SignedTransaction, index int) (uint64, error) {
	var evmC transaction.EvmContract
	var gasUsed, gasLimit uint64
	gasLimit = tx.Transaction.GasLimit * tx.Transaction.GasPrice
	if gasLimit == 0 {
		gasLimit = config.GlobalCfg.BFTConfig.GasLimit
	}
	err := json.Unmarshal(tx.Transaction.Input, &evmC)
	if err != nil {
		logger.Error("Unmarshal input error:", zap.Error(err))
		return gasUsed, err
	}

	eth_tx := evmC.EthTransaction
	TxValue := big.NewInt(0)

	deci := new(big.Int).SetUint64(ETHDECIMAL)
	TxValue = eth_tx.Value().Div(eth_tx.Value(), deci)
	bc.evm.SetConfig(TxValue, eth_tx.GasPrice(), gasLimit)

	bc.evm.Prepare(common.BytesToHash(tx.Hash()), common.BytesToHash(block.Hash), index)

	snapshotId := bc.evm.GetSnapshot()
	ethAddrs := make(map[common.Address]*big.Int)
	dump1 := bc.evm.RawDump()
	//this for sets balance to the corresponding eth address [ethAddrs] in contract.
	for addr, _ := range dump1.Accounts {
		kAddr, err := getBindingKtoAddress(DBTransaction, addr.String()) //bc.GetBindingKtoAddress(addr.String())
		if err != nil {
			continue
		}
		ka := miscellaneous.BytesSha1Address(kAddr.Bytes())
		kBalance := bc.sdb.GetBalance(ka)
		bc.sdb.SetBalance(addr, kBalance)
		ethAddrs[addr] = kBalance
	}

	var callErr error
	defer func() {
		dump2 := bc.evm.RawDump()
		//do withdraw
		if callErr == nil {
			for addr, acc := range dump2.Accounts {
				if balance, ok := big.NewInt(0).SetString(acc.Balance, 10); ok {
					kAddr, err := getBindingKtoAddress(DBTransaction, addr.String()) //bc.GetBindingKtoAddress(addr.String())
					if err != nil {
						continue
					}

					fromBalance := balance.Uint64()
					if _, ok := ethAddrs[addr]; !ok { //addr is not in map
						ka := miscellaneous.BytesSha1Address(kAddr.Bytes())
						kBalance := bc.sdb.GetBalance(ka)
						fromBalance = kBalance.Uint64() + balance.Uint64()
					}
					Frombytes := miscellaneous.E64func(fromBalance)
					if err := setBalance(bc.sdb, kAddr.Bytes(), Frombytes); err != nil {
						logger.Error("transaction setBalance error:", zap.Error(fmt.Errorf("address:%v,setbanlance:%v,error:%v", kAddr, fromBalance, err)))
						bc.evm.RevertToSnapshot(snapshotId)
						return
					}

					bc.sdb.SetBalance(addr, big.NewInt(0))
				}
			}
		} else {
			bc.evm.RevertToSnapshot(snapshotId)
		}

		for k, _ := range ethAddrs {
			delete(ethAddrs, k)
		}
	}()

	switch evmC.Operation {
	case "create", "Create":
		fmt.Println("create contract orign,code length", evmC.Origin, len(evmC.CreateCode))
		ret, contractAddr, gasLeft, err := bc.evm.Create(evmC.CreateCode, evmC.Origin)
		if err != nil {
			logger.Error("faile to create contract", zap.Error(fmt.Errorf("hash:%v,gasLeft:%v,error:%v", hex.EncodeToString(tx.Hash()), gasLeft, err))) //zap.String("name", evmC.ContractName), zap.Uint64("gasLeft", gasLeft), zap.Error(callErr))
			evmC.Ret = common.Bytes2Hex(ret)
			evmC.Status = false
			callErr = err
			return gasUsed, nil
		}

		rcfg := bc.evm.GetConfig()
		gasUsed, errs := bc.chargeGas(tx.Transaction.From, rcfg.GasLimit, gasLeft)
		if errs != nil {
			evmC.Ret = common.Bytes2Hex(ret)
			evmC.Status = false
			callErr = errs
			return gasUsed, nil
		}

		evmC.ContractAddr = contractAddr
		evmC.Status = true

		if TxValue.Uint64() > 0 {
			tx.Transaction.Amount = TxValue.Uint64()
		}
		logger.Info("Create successfully", zap.String("contract address:", contractAddr.Hex()), zap.Uint64("gasLeft", gasLeft))
	case "call", "Call":
		logger.Info("HandleContractTransaction Call", zap.String("hash", hex.EncodeToString(tx.Hash())), zap.String("contract", evmC.ContractAddr.Hex()), zap.String("input", common.Bytes2Hex(evmC.CallInput)), zap.String("origin", evmC.Origin.Hex()))
		ret, gasLeft, err := bc.evm.Call(evmC.ContractAddr, evmC.Origin, evmC.CallInput)
		if err != nil {
			logger.Error("faile to Call", zap.String("hash", hex.EncodeToString(tx.Hash())), zap.Uint64("gasLeft", gasLeft), zap.Error(err))
			evmC.Ret = common.Bytes2Hex(ret)
			evmC.Status = false
			callErr = err
			return gasUsed, nil
		}

		rcfg := bc.evm.GetConfig()
		gasUsed, errs := bc.chargeGas(tx.Transaction.From, rcfg.GasLimit, gasLeft)
		if errs != nil {
			evmC.Ret = common.Bytes2Hex(ret)
			evmC.Status = false
			callErr = errs
			return gasUsed, nil
		}

		logger.Info("Call successfully", zap.String("contract address:", evmC.ContractAddr.Hex()), zap.String("ret:", common.Bytes2Hex(ret)), zap.Uint64("gasLeft", gasLeft))
		evmC.Ret = common.Bytes2Hex(ret)
		evmC.Status = true
	}
	evmC.Logs = bc.sdb.GetLogs(common.BytesToHash(tx.Hash()), common.Hash{})
	return gasUsed, nil
}

//get storage at address
func (bc *Blockchain) GetStorageAt(addr, hash string) common.Hash {
	return bc.evm.GetStorageAt(common.HexToAddress(addr), common.HexToHash(hash))
}

//charge gasused
func (bc *Blockchain) chargeGas(origin address.Address, limit, left uint64) (uint64, error) {
	var from common.Address
	var gasUsed uint64
	_, err := bc.GetBindingEthAddress(origin)
	if err == nil {
		from = miscellaneous.BytesSha1Address(origin.Bytes())
	} else {
		from = address.KtoAddressToEthAddress(origin)
	}
	if limit >= left {
		gasUsed = limit - left
		balance := bc.sdb.GetBalance(from)

		var leftBalance uint64
		if balance.Uint64() >= gasUsed {
			leftBalance = balance.Uint64() - gasUsed
			bc.sdb.SetBalance(from, new(big.Int).SetUint64(leftBalance))
			return gasUsed, nil
		}
	}

	return gasUsed, fmt.Errorf("address[%v] Lack of balance to charge gas[%v]", from, gasUsed)
}
