package blockchain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/contract/evm"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/storage/miscellaneous"
	"github.com/korthochain/korthochain/pkg/storage/store"
	"github.com/korthochain/korthochain/pkg/transaction"
	"go.uber.org/zap"
)

func (bc *Blockchain) AddBlocks(blocks []*block.Block, limit int) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	rollroot, er := getSnapRoot(bc.db)
	if er != nil {
		log.Println("AddBlock Failed, getSnapRoot", er)
		return er
	}
	var REVERT error = nil
	defer func() {
		if REVERT != nil {
			log.Println("AddBlocks Failed,start to revert...", REVERT)
			err := bc.rollState(rollroot)
			if err != nil {
				log.Println("rollState Failed", er)
			}
			return
		}
	}()

	DBTransaction := bc.db.NewTransaction()
	defer DBTransaction.Cancel()

	for _, newblcok := range blocks {

		err := bc.AddSmallBlock(DBTransaction, newblcok)
		if err != nil {
			//logger.Error("commit ktodb", zap.Error(err), zap.Uint64("block number", newblcok.Height))
			REVERT = err
			return err
		}

	}

	_, err := factCommit(bc.sdb, true)
	if err != nil {
		logger.Error("factCommit ktodb", zap.Error(err))
		REVERT = err
		return err
	}

	if err := DBTransaction.Commit(); err != nil {
		logger.Error("commit ktodb", zap.Error(err), zap.Uint64("block number", (blocks[len(blocks)-1].Height)))
		REVERT = err
		return err
	}

	return nil

}

func (bc *Blockchain) AddSmallBlock(DBTransaction store.Transaction, block *block.Block) error {

	var REVERT error = nil
	defer func() {
		logger.SugarLogger.Info(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
		logger.SugarLogger.Infof("block.Height:%d", block.Height)
		logger.SugarLogger.Infof("nonce:%d", block.Nonce)
		logger.SugarLogger.Infof("Difficulty:%d  globalDifficulty:%d", BigToCompact(block.Difficulty), BigToCompact(block.GlobalDifficulty))
		logger.SugarLogger.Infof("hash:%s", hex.EncodeToString(block.Hash))
		logger.SugarLogger.Infof("PrevHash:%s", hex.EncodeToString(block.PrevHash))
		logger.SugarLogger.Info("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")
		logger.InfoLogger.Printf(" synchronize block success	Height=[%d]	hash=[%s]\n\n", block.Height, hex.EncodeToString(block.Hash))
		if REVERT != nil {
			logger.Error("AddBlock Failed,start to revert...", zap.Error(REVERT))
			return
		}
	}()

	var height, prevHeight uint64
	// take out the block height
	prevHeight, err := getMaxBlockHeight(DBTransaction)
	if err != nil {
		logger.Error("failed to get height", zap.Error(err))
		REVERT = err
		return err
	}

	if block.Height > InitHeight+1 {
		SnapRoothash, err := DBTransaction.Get(SnapRootKey)
		if err != nil {
			REVERT = err
			return err
		}
		startHash, err := fakeCommit1(bc.sdb, true)
		if err != nil {
			logger.Error("Failed to set factCommit", zap.Error(err))
			REVERT = err
			return err
		}

		if !bytes.Equal(SnapRoothash, startHash.Bytes()) {
			logger.Error("snaproot  is changed", zap.String("SnapRootkey-hash=[", hex.EncodeToString(SnapRoothash)),
				zap.String("],nowsnaproothash=[", hex.EncodeToString(startHash.Bytes())))
			REVERT = fmt.Errorf("snaproot not equal,old root hash:%v,current root hash:%v", hex.EncodeToString(SnapRoothash), startHash)
			return REVERT
		}
	}

	height = prevHeight + 1
	if block.Height != height {
		REVERT = fmt.Errorf("height error:current height=%d,commit height=%d", prevHeight, block.Height)
		return REVERT
	}

	// height -> hash
	hash := block.Hash
	if err := DBTransaction.Set(append(HeightPrefix, miscellaneous.E64func(height)...), hash); err != nil {
		logger.Error("Failed to set height and hash", zap.Error(err))
		REVERT = err
		return err
	}

	// reset block height
	err = DBTransaction.Set(HeightKey, miscellaneous.E64func(height))
	if err != nil {
		logger.Error("Failed to set HeightKey and hash", zap.Error(err))
		REVERT = err
		return err
	}

	bc.evm = evm.NewEvm(bc.sdb, bc.ChainCfg.ChainId, bc.ChainCfg.GasLimit, bc.ChainCfg.GasPrice)

	{
		//must set block into into evm at every addblock
		if prevHeight > 0 {
			cuurB, err := getBlockByHeight(prevHeight, DBTransaction)
			if err != nil {
				logger.Error("Failed to getBlockByHeight", zap.Error(err))
				REVERT = err
				return err
			}
			miner, _ := cuurB.Miner.NewCommonAddr()
			bc.evm.SetBlockInfo(cuurB.Height, cuurB.Timestamp, miner, cuurB.Difficulty)
		}
	}

	var blockGasU uint64
	for index, tx := range block.Transactions {
		if tx.Transaction.IsCoinBaseTransaction() {
			txHash := tx.Hash()
			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.To.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.Uint64("amount", tx.Transaction.Amount))
				REVERT = err
				return err
			}

			//	gas := tx.Transaction.GasLimit * tx.Transaction.GasPrice
			tx.GasUsed = tx.Transaction.GasLimit * tx.Transaction.GasPrice
			blockGasU += tx.GasUsed
			if err := setMinerFee(bc, block.Miner, tx.GasUsed); err != nil {
				logger.Error("Failed to set Minerfee", zap.Error(err), zap.String("from address", block.Miner.String()), zap.Uint64("fee", tx.GasUsed))
				REVERT = err
				return err
			}
			if err := bc.setToAccount(block, &tx.Transaction); err != nil {
				logger.Error("Failed to set account", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.Uint64("amount", tx.Transaction.Amount))
				REVERT = err
				return err
			}
		} else if tx.Transaction.IsLockTransaction() || tx.Transaction.IsUnlockTransaction() {
			txHash := tx.Hash()
			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.From.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				REVERT = err
				return err
			}

			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.To.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				REVERT = err
				return err
			}

			nonce := tx.Transaction.Nonce + 1
			if err := setNonce(bc.sdb, tx.Transaction.From, nonce); err != nil {
				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				REVERT = err
				return err
			}

			tx.GasUsed = tx.Transaction.GasLimit * tx.Transaction.GasPrice
			blockGasU += tx.GasUsed
			if err := setMinerFee(bc, block.Miner, tx.GasUsed); err != nil {
				logger.Error("Failed to set Minerfee", zap.Error(err), zap.String("from address", block.Miner.String()), zap.Uint64("fee", tx.GasUsed))
				REVERT = err
				return err
			}

			if tx.Transaction.IsLockTransaction() {
				if err := setFreezeAccount(bc, tx.Transaction.From, tx.Transaction.To, tx.Transaction.Amount, tx.GasUsed, 1); err != nil {
					logger.Error("Faile to setFreezeAccount", zap.String("address", tx.Transaction.From.String()),
						zap.Uint64("amount", tx.Transaction.Amount))
					REVERT = err
					return err
				}
			} else {
				if err := setFreezeAccount(bc, tx.Transaction.From, tx.Transaction.To, tx.Transaction.Amount, tx.GasUsed, 0); err != nil {
					logger.Error("Faile to setFreezeAccount", zap.String("address", tx.Transaction.From.String()),
						zap.Uint64("amount", tx.Transaction.Amount))
					REVERT = err
					return err
				}
			}

		} else if tx.Transaction.IsEvmContractTransaction() {
			txHash := tx.Hash()
			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.From.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("hash", transaction.HashToString(txHash)))
				REVERT = err
				return err
			}

			gasLeft, err := bc.handleContractTransaction(block, DBTransaction, tx, index)
			if err != nil {
				logger.Error("Failed to HandleContractTransaction", zap.Error(err), zap.String("hash", transaction.HashToString(txHash)))
				REVERT = err
				return err
			}

			evmcfg := bc.evm.GetConfig()
			if evmcfg.GasLimit < gasLeft {
				logger.Error("Failed to HandleContractTransaction", zap.Error(fmt.Errorf("hash[%v],evm gaslimit[%v] < gasLeft[%v]", transaction.HashToString(txHash), evmcfg.GasLimit, gasLeft)))
				REVERT = fmt.Errorf("error: hash[%v],evm gaslimit[%v] < gasLeft[%v]", transaction.HashToString(txHash), evmcfg.GasLimit, gasLeft)
				return REVERT
			}
			tx.GasUsed = evmcfg.GasLimit - gasLeft
			blockGasU += tx.GasUsed

			if err := setMinerFee(bc, block.Miner, tx.GasUsed); err != nil {
				logger.Error("Failed to set Minerfee", zap.Error(err), zap.String("hash", transaction.HashToString(txHash)), zap.Uint64("gasUsed", tx.GasUsed))
				REVERT = err
				return err
			}

			// update balance
			if err := setAccount(bc, tx); err != nil {
				logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				REVERT = err
				return err
			}
		} else {

			txHash := tx.Hash()
			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.From.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				REVERT = err
				return err
			}

			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.To.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				REVERT = err
				return err
			}
			// update nonce,txs in block must be ordered
			nonce := tx.Transaction.Nonce + 1
			if err := setNonce(bc.sdb, tx.Transaction.From, nonce); err != nil {
				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				REVERT = err
				return err
			}

			//err

			tx.GasUsed = tx.Transaction.GasLimit * tx.Transaction.GasPrice
			blockGasU += tx.GasUsed

			if err := setMinerFee(bc, block.Miner, tx.GasUsed); err != nil {
				logger.Error("Failed to set Minerfee", zap.Error(err), zap.String("from address", block.Miner.String()), zap.Uint64("fee", tx.GasUsed))
				REVERT = err
				return err
			}

			// update balance
			if err := setAccount(bc, tx); err != nil {
				logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				REVERT = err
				return err
			}
		}
	}

	t0 := time.Now()
	comHash, err := fakeCommit1(bc.sdb, true)
	if err != nil {
		logger.Error("Failed to set factCommit", zap.Error(err))
		REVERT = err
		return err
	}
	logger.Info("sub factcommit", zap.Float64("second", time.Since(t0).Seconds()))
	if block.Height > InitHeight+1 {
		oldSnapRootkey, err := DBTransaction.Get(SnapRootKey)
		if err != nil {
			REVERT = err
			return err
		}

		logger.Info("AddBlock", zap.String("oldSnapRootkey", hex.EncodeToString(oldSnapRootkey)))
		logger.Info("AddBlock", zap.String("b.SnapRootkey", hex.EncodeToString(block.SnapRoot)))
		logger.Info("AddBlock", zap.String("newSnapRootkey", hex.EncodeToString(comHash.Bytes())))

		if bytes.Equal(oldSnapRootkey, block.SnapRoot) {
			block.SnapRoot = comHash.Bytes()
		} else if !bytes.Equal(comHash.Bytes(), block.SnapRoot) {
			REVERT = fmt.Errorf("SnapRoot not equal")
			return REVERT
		}
	} else if block.Height == InitHeight+1 {
		block.SnapRoot = comHash.Bytes()
	} else {
		REVERT = fmt.Errorf("block.height[%d]<initheight[%d]", block.Height, InitHeight)
		return REVERT
	}

	// hash -> block
	block.GasUsed = blockGasU
	data, err := block.Serialize()
	if err != nil {
		logger.Error("failed serialize block", zap.Error(err))
		REVERT = err
		return err
	}

	if err := DBTransaction.Set(hash, data); err != nil {
		logger.Error("Failed to set block", zap.Error(err))
		REVERT = err
		return err
	}

	if err := DBTransaction.Set(append(SnapRootPrefix, miscellaneous.E64func(height)...), comHash.Bytes()); err != nil {
		logger.Error("Failed to set height and hash", zap.Error(err))
		REVERT = err
		return err
	}
	err = DBTransaction.Set(SnapRootKey, comHash.Bytes())
	if err != nil {
		logger.Error("Failed to set height and hash", zap.Error(err))
		REVERT = err
		return err
	}

	return nil
}

func fakeCommit1(sdb *state.StateDB, deleteEmptyObjects bool) (common.Hash, error) {
	ha, err := sdb.Commit(deleteEmptyObjects)
	if err != nil {
		logger.Error("stateDB commit error")
		return common.Hash{}, err
	}

	return ha, err
}
