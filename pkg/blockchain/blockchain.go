package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"korthochain/pkg/contract/evm"

	"github.com/dgraph-io/badger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/storage/merkle"
	"github.com/korthochain/korthochain/pkg/storage/miscellaneous"
	"github.com/korthochain/korthochain/pkg/storage/store"
	"github.com/korthochain/korthochain/pkg/storage/store/bg"
	"github.com/korthochain/korthochain/pkg/storage/store/bg/bgdb"
	"github.com/korthochain/korthochain/pkg/transaction"
	"go.uber.org/zap"
)

const (
	MAXUINT64 = ^uint64(0)
)

var (
	// SnapRootKey key to store snaproot in database
	SnapRootKey = []byte("snapRoot")
	// HeightKey key to store height in database
	HeightKey = []byte("height")
	// FreezeKey store the map name of freeze balance
	FreezeKey = []byte("freeze")
)

var (
	// SnapRootPrefix prefix of block snapRoot
	SnapRootPrefix = []byte("blockSnap")
	// HeightPrefix prefix of block height key
	HeightPrefix = []byte("blockheight")
)

// Blockchain blockchain data structure
type Blockchain struct {
	mu  sync.RWMutex
	db  store.DB
	sdb *state.StateDB
	evm *evm.Evm
}

// TxIndex transaction data index structure
type TxIndex struct {
	Height uint64
	Index  uint64
}

// New create blockchain object
func New(db *badger.DB) (*Blockchain, error) {
	bgs := bg.New(db)
	cdb := bgdb.NewBadgerDatabase(bgs)
	sdb := state.NewDatabase(cdb)
	root, err := getSnapRoot(bgs)
	if err != nil {
		logger.Error("failed to getSnapRoot")
		return nil, err
	}
	stdb, err := state.New(root, sdb, nil)
	if err != nil {
		logger.Error("failed to new state")
		return nil, err
	}

	bc := &Blockchain{db: bgs, sdb: stdb}
	evm.NewEvm(stdb, bc)
	return bc, nil
}

// setNonce set address nonce
func setNonce(s *state.StateDB, addr, nonce []byte) error {
	a := miscellaneous.BytesSha1Address(addr)
	n, err := miscellaneous.D64func(nonce)
	if err != nil {
		logger.Error("error from miscellaneous.D64func")
		return err
	}
	s.SetNonce(a, n)
	return nil
}

// setBalance set address balance
func setBalance(s *state.StateDB, addr, balance []byte) error {
	a := miscellaneous.BytesSha1Address(addr)
	balanceU, err := miscellaneous.D64func(balance)

	if err != nil {
		logger.Error("error from miscellaneous.D64func")
		return err
	}
	s.SetBalance(a, new(big.Int).SetUint64(balanceU))
	return nil
}

// setFreezeBalance set address freeze balance
func setFreezeBalance(s *state.StateDB, addr, freezeBal []byte) error {
	ak := miscellaneous.EMapKey(FreezeKey, addr)
	a := miscellaneous.BytesSha1Address(ak)

	freezeBalU, err := miscellaneous.D64func(freezeBal)
	if err != nil {
		logger.Error("error from miscellaneous.D64func")
		return err
	}
	s.SetBalance(a, new(big.Int).SetUint64(freezeBalU))
	return nil
}

// setAccount set the balance of the corresponding account
func setAccount(sdb *state.StateDB, tx *transaction.SignedTransaction) error {
	from, to := tx.Transaction.From.Bytes(), tx.Transaction.To.Bytes()

	fromCA := miscellaneous.BytesSha1Address(from)
	fromBalBig := sdb.GetBalance(fromCA)
	fromBalance := fromBalBig.Uint64()

	gas := tx.Transaction.GasLimit * tx.Transaction.GasPrice
	hash := tx.Hash()
	if fromBalance < tx.Transaction.Amount+gas {
		return fmt.Errorf("not sufficient funds,hash:%s,from balance(%d) < amount(%d) + gas(%d)",
			hex.EncodeToString(hash), fromBalance, tx.Transaction.Amount, gas)
	}
	fromBalance -= tx.Transaction.Amount + gas

	toCA := miscellaneous.BytesSha1Address(to)
	tobalance := sdb.GetBalance(toCA)
	toBalance := tobalance.Uint64()
	if MAXUINT64-toBalance-gas < tx.Transaction.Amount {
		return fmt.Errorf("amount is too large,hash:%s,max int64(%d)-balance(%d)-gas(%d) < amount(%d)", hash, MAXUINT64, toBalance, gas, tx.Transaction.Amount)
	}
	toBalance += tx.Transaction.Amount

	Frombytes := miscellaneous.E64func(fromBalance)
	Tobytes := miscellaneous.E64func(toBalance)

	err := setBalance(sdb, from, Frombytes)
	if err != nil {
		return err
	}
	err = setBalance(sdb, to, Tobytes)
	if err != nil {
		return err
	}

	return nil
}

// setToAccount set the balance of the specified account
func (bc *Blockchain) setToAccount(DBTransaction store.Transaction, block *block.Block, tx *transaction.Transaction) error {
	to := tx.To.Bytes()
	toCA := miscellaneous.BytesSha1Address(to)

	toBalanceBig := bc.sdb.GetBalance(toCA)
	balance := toBalanceBig.Uint64()

	if MAXUINT64-tx.Amount < balance {
		return fmt.Errorf("not sufficient funds")
	}
	newBalanceBytes := miscellaneous.E64func(balance + tx.Amount)

	err := bc.handleMinedPledge(DBTransaction, block, tx.To, tx.Amount)
	if err != nil {
		return err
	}

	return setBalance(bc.sdb, tx.To.Bytes(), newBalanceBytes)
}

// setMinerFee set the balance of the miner's account
func setMinerFee(sdb *state.StateDB, to []byte, amount uint64) error {
	toCA := miscellaneous.BytesSha1Address(to)
	tobalance := sdb.GetBalance(toCA)
	toBalance := tobalance.Uint64()

	if MAXUINT64-toBalance < amount {
		return fmt.Errorf("amount is too large,max int64(%d)-balance(%d) < amount(%d)", MAXUINT64, toBalance, amount)
	}
	toBalanceBytes := miscellaneous.E64func(toBalance + amount)

	return setBalance(sdb, to, toBalanceBytes)
}

// GetBalance get the balance of the address
func (bc *Blockchain) GetBalance(address []byte) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getBalance(address)
}

// getBalance get the balance of the address
func (bc *Blockchain) getBalance(address []byte) (uint64, error) {
	addr := miscellaneous.BytesSha1Address(address)
	balanceBig := bc.sdb.GetBalance(addr)
	return balanceBig.Uint64(), nil
}

// GetFreezeBalance get the freeze balance of the address
func (bc *Blockchain) GetFreezeBalance(address []byte) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getFreezeBalance(address)
}

// getFreezeBalance get the freeze balance of the address
func (bc *Blockchain) getFreezeBalance(address []byte) (uint64, error) {
	ak := miscellaneous.EMapKey(FreezeKey, address)
	freezeAddr := miscellaneous.BytesSha1Address(ak)

	freezeBalBytes := bc.sdb.GetBalance(freezeAddr)
	return freezeBalBytes.Uint64(), nil
}

// getFreezeBalance get the freeze balance of the address
func getFreezeBalance(bc *state.StateDB, address []byte) (uint64, error) {
	ak := miscellaneous.EMapKey(FreezeKey, address)
	freezeAddr := miscellaneous.BytesSha1Address(ak)
	freezeBalBytes := bc.GetBalance(freezeAddr)

	return freezeBalBytes.Uint64(), nil
}

// GetNonce get the nonce of the address
func (bc *Blockchain) GetNonce(address []byte) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getNonce(address)
}

// getNonce get the nonce of the address
func (bc *Blockchain) getNonce(address []byte) (uint64, error) {
	var initNonce uint64 = 1
	addr := miscellaneous.BytesSha1Address(address)
	n := bc.sdb.GetNonce(addr)
	if n == 0 {
		return initNonce, setNonce(bc.sdb, address, miscellaneous.E64func(initNonce))
	}
	return n, nil
}

// getSnapRoot Get the SnapRoot of the DB
func getSnapRoot(db store.DB) (common.Hash, error) {
	sr, err := db.Get(SnapRootKey)
	if err == store.NotExist {
		return common.Hash{}, nil
	} else if err != nil {
		return common.Hash{}, err
	}
	return common.BytesToHash(sr), nil
}

// factCommit writes the state to the underlying in-memory trie database
func factCommit(sdb *state.StateDB, deleteEmptyObjects bool) (common.Hash, error) {
	ha, err := sdb.Commit(deleteEmptyObjects)
	if err != nil {
		logger.Error("stateDB commit error")
		return common.Hash{}, err
	}
	triDB := sdb.Database().TrieDB()
	err = triDB.Commit(ha, true, nil)
	if err != nil {
		logger.Error("triDB commit error")
		return ha, err
	}

	return ha, err
}

// GetHash get the hash corresponding to the block height
func (bc *Blockchain) GetHash(height uint64) (hash []byte, err error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.db.Get(append(HeightPrefix, miscellaneous.E64func(height)...))
}

// GetMaxBlockHeight get maximum block height
func (bc *Blockchain) GetMaxBlockHeight() (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	heightBytes, err := bc.db.Get(HeightKey)
	if err == store.NotExist {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return miscellaneous.D64func(heightBytes)
}

// GetHeight Gets the current block height
func (bc *Blockchain) GetHeight() (height uint64, err error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getHeight()
}

// getHeight Gets the current block height
func (bc *Blockchain) getHeight() (uint64, error) {
	heightBytes, err := bc.db.Get(HeightKey)
	if err == store.NotExist {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return miscellaneous.D64func(heightBytes)
}

// GetBlockByHeight get the block corresponding to the block height
func (bc *Blockchain) GetBlockByHeight(height uint64) (*block.Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getBlockByheight(height)
}

// getBlockByHeight get the block corresponding to the block height
func (bc *Blockchain) getBlockByheight(height uint64) (*block.Block, error) {
	if height < 1 {
		return nil, errors.New("parameter error")
	}
	//Get the hash first
	hash, err := bc.db.Get(append(HeightPrefix, miscellaneous.E64func(height)...))
	if err != nil {
		return nil, err
	}
	//Then get the block through hash
	blockData, err := bc.db.Get(hash)
	if err != nil {
		return nil, err
	}

	return block.Deserialize(blockData)
}

// GetTransactionByHash get the transaction corresponding to the transaction hash
func (bc *Blockchain) GetTransactionByHash(hash []byte) (*transaction.Transaction, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getTransactionByHash(hash)
}

// getTransactionByHash get the transaction corresponding to the transaction hash
func (bc *Blockchain) getTransactionByHash(hash []byte) (*transaction.Transaction, error) {

	Hi, err := bc.db.Get(hash)
	if err != nil {
		logger.Error("failed to get hash", zap.Error(err))
		return nil, err
	}
	var txindex TxIndex
	err = json.Unmarshal(Hi, &txindex)
	if err != nil {
		logger.Error("Failed to unmarshal bytes", zap.Error(err))
		return nil, err
	}
	b, err := bc.getBlockByheight(txindex.Height)
	if err != nil {
		logger.Error("failed to getblock height", zap.Error(err), zap.Uint64("height", txindex.Height))
		return nil, err
	}

	tx := &b.Transactions[txindex.Index].Transaction

	return tx, nil
}

// NewBlock create a new block for the blockchain
func (bc *Blockchain) NewBlock(txs []*transaction.SignedTransaction, minaddr address.Address) (*block.Block, error) {
	logger.Info("start to new block")
	var height, prevHeight uint64
	var prevHash []byte
	var gasUsed uint64
	prevHeight, err := bc.GetHeight()
	if err != nil {
		logger.Error("failed to get height", zap.Error(err))
		return nil, err
	}

	height = prevHeight + 1
	if height > 1 {
		prevHash, err = bc.GetHash(prevHeight)
		if err != nil {
			logger.Error("failed to get hash", zap.Error(err), zap.Uint64("previous height", prevHeight))
			return nil, err
		}
	} else {
		prevHash = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	}

	// Currency distribution
	txs = Distr(txs, minaddr, height)

	// Generate Merkel root, if there is no deal, calling GetMthash will painc
	txBytesList := make([][]byte, 0, len(txs))
	for _, tx := range txs {
		serialize, _ := tx.Serialize()
		txBytesList = append(txBytesList, serialize)
		gasUsed += tx.Transaction.GasLimit * tx.Transaction.GasPrice
	}
	tree := merkle.New(sha256.New(), txBytesList)
	root := tree.GetMtHash()
	getRoot, _ := getSnapRoot(bc.db)
	snapRoot := getRoot.Bytes()

	difficulty := new(big.Int)

	block := &block.Block{
		Height:       height,
		PrevHash:     prevHash,
		Transactions: txs,
		Root:         root,
		Version:      1,
		Timestamp:    uint64(time.Now().Unix()),
		Miner:        minaddr,
		SnapRoot:     snapRoot,
		Difficulty:   difficulty,
		Nonce:        1,
		GasLimit:     1,
		GasUsed:      gasUsed,
	}
	block.SetHash()
	logger.Info("end to new block")
	return block, nil
}

// AddBlock add blocks to blockchain
func (bc *Blockchain) AddBlock(block *block.Block) error {
	logger.Info("addBlock", zap.Uint64("blockHeight", block.Height))
	bc.mu.Lock()
	defer bc.mu.Unlock()

	DBTransaction := bc.db.NewTransaction()
	defer DBTransaction.Cancel()
	var err error
	var height, prevHeight uint64
	// take out the block height
	prevHeight, err = bc.getHeight()
	if err != nil {
		logger.Error("failed to get height", zap.Error(err))
		return err
	}

	height = prevHeight + 1
	if block.Height != height {
		return fmt.Errorf("height error:current height=%d,commit height=%d", prevHeight, block.Height)
	}

	// height -> hash
	hash := block.Hash
	if err = DBTransaction.Set(append(HeightPrefix, miscellaneous.E64func(height)...), hash); err != nil {
		logger.Error("Failed to set height and hash", zap.Error(err))
		return err
	}

	// reset block height
	DBTransaction.Del(HeightKey)
	DBTransaction.Set(HeightKey, miscellaneous.E64func(height))

	for index, tx := range block.Transactions {
		if tx.Transaction.IsCoinBaseTransaction() {
			txHash := tx.Hash()
			if err = setTxbyaddrKV(DBTransaction, tx.Transaction.To.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.Uint64("amount", tx.Transaction.Amount))
				return err
			}

			gas := tx.Transaction.GasLimit * tx.Transaction.GasPrice
			if err = setMinerFee(bc.sdb, block.Miner.Bytes(), gas); err != nil {
				logger.Error("Failed to set Minerfee", zap.Error(err), zap.String("from address", block.Miner.String()), zap.Uint64("fee", gas))
				return err
			}

			if err := bc.setToAccount(DBTransaction, block, &tx.Transaction); err != nil {
				logger.Error("Failed to set account", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.Uint64("amount", tx.Transaction.Amount))
				return err
			}
		} else if tx.Transaction.IsLockTransaction() || tx.Transaction.IsUnlockTransaction() {
			txHash := tx.Hash()
			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.From.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				return err
			}

			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.To.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				return err
			}

			nonce := tx.Transaction.Nonce + 1
			if err := setNonce(bc.sdb, tx.Transaction.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				return err
			}
			var frozenBalBytes []byte
			frozenBal, _ := getFreezeBalance(bc.sdb, tx.Transaction.To.Bytes())
			if tx.Transaction.IsLockTransaction() {
				frozenBalBytes = miscellaneous.E64func(tx.Transaction.Amount + frozenBal)
			} else {
				frozenBalBytes = miscellaneous.E64func(frozenBal - tx.Transaction.Amount)
			}

			gas := tx.Transaction.GasLimit * tx.Transaction.GasPrice
			if err = setMinerFee(bc.sdb, block.Miner.Bytes(), gas); err != nil {
				logger.Error("Failed to set Minerfee", zap.Error(err), zap.String("from address", block.Miner.String()), zap.Uint64("fee", gas))
				return err
			}

			if err := setFreezeBalance(bc.sdb, tx.Transaction.To.Bytes(), frozenBalBytes); err != nil {
				logger.Error("Faile to freeze balance", zap.String("address", tx.Transaction.To.String()),
					zap.Uint64("amount", tx.Transaction.Amount))
				return err
			}
		} else if tx.Transaction.IsPledgeTrasnaction() {
			txHash := tx.Hash()
			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.From.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("hash", transaction.HashToString(txHash)))
				return err
			}
			nonce := tx.Transaction.Nonce + 1
			if err := setNonce(bc.sdb, tx.Transaction.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
				logger.Error("Failed to set nonce", zap.Error(err), zap.String("hash", transaction.HashToString(txHash)))
				return err
			}

			err := bc.handlePledgeTransaction(block, DBTransaction, &tx.Transaction)
			if err != nil {
				logger.Error("Failed to Pledge", zap.Error(err), zap.String("hash", transaction.HashToString(txHash)))
				return err
			}

			gasUsed := tx.Transaction.GasLimit * tx.Transaction.GasPrice
			if err = setMinerFee(bc.sdb, block.Miner.Bytes(), gasUsed); err != nil {
				logger.Error("Failed to set Minerfee", zap.Error(err), zap.String("hash", transaction.HashToString(txHash)), zap.Uint64("gasUsed", gasUsed))
				return err
			}
		} else if tx.Transaction.IsEvmContractTransaction() {
			txHash := tx.Hash()
			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.From.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("hash", transaction.HashToString(txHash)))
				return err
			}

			nonce := tx.Transaction.Nonce + 1
			if err := setNonce(bc.sdb, tx.Transaction.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
				logger.Error("Failed to set nonce", zap.Error(err), zap.String("hash", transaction.HashToString(txHash)))
				return err
			}

			gasUsed, err := bc.handleContractTransaction(block, DBTransaction, tx, index)
			if err != nil {
				logger.Error("Failed to HandleContractTransaction", zap.Error(err), zap.String("hash", transaction.HashToString(txHash)))
				return err
			}

			//gasUsed := tx.Transaction.GasLimit * tx.Transaction.GasPrice
			if err = setMinerFee(bc.sdb, block.Miner.Bytes(), gasUsed); err != nil {
				logger.Error("Failed to set Minerfee", zap.Error(err), zap.String("hash", transaction.HashToString(txHash)), zap.Uint64("gasUsed", gasUsed))
				return err
			}
		} else {
			txHash := tx.Hash()
			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.From.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				return err
			}

			if err := setTxbyaddrKV(DBTransaction, tx.Transaction.To.Bytes(), txHash, height, uint64(index)); err != nil {
				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				return err
			}
			// update nonce,txs in block must be ordered
			nonce := tx.Transaction.Nonce + 1
			if err := setNonce(bc.sdb, tx.Transaction.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				return err
			}

			gas := tx.Transaction.GasLimit * tx.Transaction.GasPrice
			if err = setMinerFee(bc.sdb, block.Miner.Bytes(), gas); err != nil {
				logger.Error("Failed to set Minerfee", zap.Error(err), zap.String("from address", block.Miner.String()), zap.Uint64("fee", gas))
				return err
			}

			// update balance
			if err := setAccount(bc.sdb, tx); err != nil {
				logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
					zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
				return err
			}
		}
	}

	{
		//set block into into evm
		bc.evm.SetBlockInfo(block.Height, block.Miner.String(), uint64(block.Timestamp))
	}

	{
		//release 70% Mined
		err := bc.releaseMined(DBTransaction, block)
		if err != nil {
			logger.Error("Failed to Release Mined", zap.Error(err))
			return err
		}
	}
	{
		//release pledge
		err := bc.releasePlege(DBTransaction, block)
		if err != nil {
			logger.Error("Failed to Release Plege", zap.Error(err))
			return err
		}
	}

	// hash -> block
	if err = DBTransaction.Set(hash, block.Serialize()); err != nil {
		logger.Error("Failed to set block", zap.Error(err))
		return err
	}

	{
		comHash, err := factCommit(bc.sdb, true)
		if err != nil {
			logger.Error("Failed to set factCommit", zap.Error(err))
			return err
		}
		if err = DBTransaction.Set(append(SnapRootPrefix, miscellaneous.E64func(height)...), comHash.Bytes()); err != nil {
			logger.Error("Failed to set height and hash", zap.Error(err))
			return err
		}
		DBTransaction.Del(SnapRootKey)
		DBTransaction.Set(SnapRootKey, comHash.Bytes())

		if err := DBTransaction.Commit(); err != nil {
			logger.Error("commit ktodb", zap.Error(err), zap.Uint64("block number", block.Height))
			return err
		}
	}
	logger.Info("End addBlock", zap.Uint64("blockHeight", block.Height))
	return nil
}

// DeleteBlock delete some blocks from the blockchain
func (bc *Blockchain) DeleteBlock(height uint64) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	dbHeight, err := bc.getHeight()
	if err != nil {
		logger.Error("failed to get height", zap.Error(err))
		return err
	}

	if height > dbHeight {
		return fmt.Errorf("Wrong height to delete,[%v] should <= current height[%v]", height, dbHeight)
	}

	for dH := dbHeight; dH >= height; dH-- {

		DBTransaction := bc.db.NewTransaction()

		logger.Info("Start to delete block", zap.Uint64("height", dH))
		block, err := bc.getBlockByheight(dH)
		if err != nil {
			logger.Error("failed to get block", zap.Error(err))
			return err
		}

		for i, tx := range block.Transactions {
			if tx.Transaction.IsCoinBaseTransaction() {
				if err = deleteTxbyaddrKV(DBTransaction, tx.Transaction.To.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
						zap.Uint64("amount", tx.Transaction.Amount))
					return err
				}

			} else {
				if tx.Transaction.IsLockTransaction() || tx.Transaction.IsUnlockTransaction() {
					if err := deleteTxbyaddrKV(DBTransaction, tx.Transaction.From.Bytes(), *tx, uint64(i)); err != nil {
						logger.Error("Failed to del transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
							zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
						return err
					}

					if err := deleteTxbyaddrKV(DBTransaction, tx.Transaction.To.Bytes(), *tx, uint64(i)); err != nil {
						logger.Error("Failed to del transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
							zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
						return err
					}
				}

				// errUpdate
				if err := deleteTxbyaddrKV(DBTransaction, tx.Transaction.From.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to del transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
						zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
					return err
				}

				if err := deleteTxbyaddrKV(DBTransaction, tx.Transaction.To.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to del transaction", zap.Error(err), zap.String("from address", tx.Transaction.From.String()),
						zap.String("to address", tx.Transaction.To.String()), zap.Uint64("amount", tx.Transaction.Amount))
					return err
				}
			}
		}

		// height -> hash
		hash := block.Hash
		if err = DBTransaction.Del(append(HeightPrefix, miscellaneous.E64func(block.Height)...)); err != nil {
			logger.Error("Failed to Del height and hash", zap.Error(err))
			return err
		}
		// hash -> block
		if err = DBTransaction.Del(hash); err != nil {
			logger.Error("Failed to Del block", zap.Error(err))
			return err
		}
		// process snapshot
		sn, _ := DBTransaction.Get(append(SnapRootPrefix, miscellaneous.E64func(block.Height-1)...))
		if err = DBTransaction.Del(append(SnapRootPrefix, miscellaneous.E64func(block.Height)...)); err != nil {
			return err
		}

		DBTransaction.Set(SnapRootKey, sn)
		DBTransaction.Set(HeightKey, miscellaneous.E64func(dH-1))
		if err := DBTransaction.Commit(); err != nil {
			logger.Error("DBTransaction Commit err", zap.Error(err))
			return err
		}

		DBTransaction.Cancel()
	}
	logger.Info("End delete")
	return nil
}

// Distr coin out test
func Distr(txs []*transaction.SignedTransaction, minaddr address.Address, height uint64) []*transaction.SignedTransaction {
	var total uint64 = 170000000000
	x := height / 3153600
	for i := 0; uint64(i) < x; i++ {
		total = total * 5 / 10
	}

	from := new(address.Address)
	txm := transaction.Transaction{
		From:   *from,
		To:     minaddr,
		Nonce:  0,
		Amount: total,
		Type:   transaction.CoinBaseTransaction,
	}
	stxm := transaction.SignedTransaction{
		Transaction: txm,
	}
	txs = append(txs, &stxm)
	return txs
}

// setTxbyaddrKV transaction data is stored by address and corresponding kV
func setTxbyaddrKV(DBTransaction store.Transaction, addr []byte, hash []byte, height, index uint64) error {
	DBTransaction.Mset(addr, hash, []byte(""))
	txindex := &TxIndex{
		Height: height,
		Index:  index,
	}

	tdex, err := json.Marshal(txindex)
	if err != nil {
		logger.Error("Failed Marshal txindex", zap.Error(err))
		return err
	}
	DBTransaction.Set(hash, tdex)

	return err
}

// deleteTxbyaddrKV delete transaction data by address and corresponding kV
func deleteTxbyaddrKV(DBTransaction store.Transaction, addr []byte, tx transaction.SignedTransaction, index uint64) error {
	txHash := tx.Hash()
	err := DBTransaction.Mdel(addr, txHash)
	if err != nil {
		logger.Error("deleteTxbyaddrKV Mdel err ", zap.Error(err))
		return err
	}

	if err := DBTransaction.Del(txHash); err != nil {
		logger.Error("deleteTxbyaddrKV Del err ", zap.Error(err))
		return err
	}

	return err
}
