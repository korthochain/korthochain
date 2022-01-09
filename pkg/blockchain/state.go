package blockchain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/storage/store"
	"github.com/korthochain/korthochain/pkg/transaction"
	"go.uber.org/zap"
)

// setNonce set address nonce
func setNonce(s *state.StateDB, addr address.Address, nonce uint64) error {

	coAddr, err := addr.NewCommonAddr()
	if err != nil {
		logger.Error("NewCommonAddr:", zap.Error(err), zap.String("address", addr.String()))
		return err
	}

	sdbNonce := s.GetNonce(coAddr)
	if sdbNonce == 0 {
		sdbNonce = 1
	}
	if sdbNonce+1 != nonce {
		return fmt.Errorf("error nonce(%v) != dbNonce(%v)", nonce, sdbNonce+1)
	}

	s.SetNonce(coAddr, nonce)
	return nil
}

// setBalance set address balance
func setBalance(s *state.StateDB, addr address.Address, balance *big.Int) error {
	coAddr, err := addr.NewCommonAddr()
	if err != nil {
		return err
	}
	s.SetBalance(coAddr, balance)
	return nil
}

// setAccount set the balance of the corresponding account
func setAccount(bc *Blockchain, tx *transaction.FinishedTransaction, defaultAmount ...uint64) error {
	if tx.Type == transaction.WithdrawToEthTransaction {
		DBTransaction := bc.db.NewTransaction()
		defer DBTransaction.Cancel()

		hash := tx.Hash()

		cmaddr, err := tx.Transaction.From.NewCommonAddr()
		if err != nil {
			return err
		}

		kaddr, err := getBindingKtoAddress(DBTransaction, cmaddr.Hex())
		if err != nil {
			return err
		}

		fromBalance, err := bc.getAvailableBalance(kaddr)
		if err != nil {
			return err
		}

		if fromBalance < tx.Transaction.Amount+tx.GasUsed {
			return fmt.Errorf("fromBalance insufficient, fromBalance(%d)<amount(%d)+gas(%d) ", fromBalance, tx.Transaction.Amount, tx.GasUsed)
		}

		toBalance, err := bc.getBalance(tx.Transaction.To)
		if err != nil {
			return err
		}

		if fromBalance < tx.Transaction.Amount+tx.GasUsed {
			return fmt.Errorf("not sufficient funds,hash:%s,from balance(%d) < amount(%d) + gas(%d)",
				hex.EncodeToString(hash), fromBalance, tx.Transaction.Amount, tx.GasUsed)
		}
		fromBalance -= tx.Transaction.Amount + tx.GasUsed

		if MAXUINT64-toBalance-tx.GasUsed < tx.Transaction.Amount {
			return fmt.Errorf("amount is too large,hash:%s,max int64(%d)-balance(%d)-gas(%d) < amount(%d)", hash, MAXUINT64, toBalance, tx.GasUsed, tx.Transaction.Amount)
		}
		toBalance += tx.Transaction.Amount

		err = setBalance(bc.sdb, kaddr, Uint64ToBigInt(fromBalance))
		if err != nil {
			return err
		}
		err = setBalance(bc.sdb, tx.Transaction.To, Uint64ToBigInt(toBalance))
		if err != nil {
			return err
		}

		return nil
	}

	from, to := tx.Transaction.From, tx.Transaction.To
	if tx.Type != transaction.PledgeTrasnaction {
		availableBalance, err := bc.getAvailableBalance(from)
		if err != nil {
			return err
		}
		if availableBalance < tx.Transaction.Amount+tx.GasUsed {
			return fmt.Errorf("availableBalance insufficient, availableBalance(%d)<amount(%d)+gas(%d) ", availableBalance, tx.Transaction.Amount, tx.GasUsed)
		}
	}

	if bytes.Equal(from.Bytes(), to.Bytes()) == true || tx.Transaction.IsPledgeTrasnaction() || tx.Transaction.IsEvmContractTransaction() {
		hash := tx.Hash()
		fromBalance, err := bc.getBalance(from)
		if err != nil {
			return err
		}

		if fromBalance < tx.GasUsed {
			return fmt.Errorf("not sufficient funds,hash:%s,from balance(%d) < gas(%d)",
				hex.EncodeToString(hash), fromBalance, tx.GasUsed)
		}
		if MAXUINT64-fromBalance < tx.GasUsed {
			return fmt.Errorf("gas is too large,hash:%s,max int64(%d)-balance(%d)<gas(%d) ", hash, MAXUINT64, fromBalance, tx.GasUsed)
		}

		fromBalance -= tx.GasUsed
		err = setBalance(bc.sdb, from, new(big.Int).SetUint64(fromBalance))
		if err != nil {
			return err
		}
	} else if tx.Transaction.IsPledgeBreakTransaction() {
		frBalance, err := bc.getBalance(from)
		if err != nil {
			return err
		}
		if tx.Amount != 0 {
			return fmt.Errorf("Amount Expected 0, Amount(%d)", tx.Amount)
		}
		if frBalance < defaultAmount[0]+tx.GasUsed {
			return fmt.Errorf("availableBalance insufficient, frBalance(%d)<defaultAmount(%d)+gas(%d) ", frBalance, defaultAmount[0], tx.GasUsed)
		}
		defaultBalance := frBalance - defaultAmount[0] - tx.GasUsed
		err = setBalance(bc.sdb, from, new(big.Int).SetUint64(defaultBalance))
		if err != nil {
			return err
		}

	} else {
		hash := tx.Hash()
		fromBalance, err := bc.getBalance(from)
		if err != nil {
			return err
		}
		toBalance, err := bc.getBalance(to)
		if err != nil {
			return err
		}

		if fromBalance < tx.Transaction.Amount+tx.GasUsed {
			return fmt.Errorf("not sufficient funds,hash:%s,from balance(%d) < amount(%d) + gas(%d)",
				hex.EncodeToString(hash), fromBalance, tx.Transaction.Amount, tx.GasUsed)
		}
		fromBalance -= tx.Transaction.Amount + tx.GasUsed

		if MAXUINT64-toBalance-tx.GasUsed < tx.Transaction.Amount {
			return fmt.Errorf("amount is too large,hash:%s,max int64(%d)-balance(%d)-gas(%d) < amount(%d)", hash, MAXUINT64, toBalance, tx.GasUsed, tx.Transaction.Amount)
		}
		toBalance += tx.Transaction.Amount

		err = setBalance(bc.sdb, from, new(big.Int).SetUint64(fromBalance))
		if err != nil {
			return err
		}
		err = setBalance(bc.sdb, to, new(big.Int).SetUint64(toBalance))
		if err != nil {
			return err
		}
	}
	return nil
}

// setToAccount set the balance of the specified account
func (bc *Blockchain) setToAccount(block *block.Block, tx *transaction.Transaction) error {
	toBalance, err := bc.getBalance(tx.To)
	if err != nil {
		return err
	}

	if MAXUINT64-tx.Amount < toBalance {
		return fmt.Errorf("not sufficient funds")
	}
	return setBalance(bc.sdb, tx.To, new(big.Int).SetUint64(toBalance+tx.Amount))
}

// setMinerFee set the balance of the miner's account
func setMinerFee(bc *Blockchain, to address.Address, gasAmount uint64) error {
	toBalance, err := bc.getBalance(to)
	if err != nil {
		return err
	}

	if MAXUINT64-toBalance < gasAmount {
		return fmt.Errorf("amount is too large,max int64(%d)-balance(%d) < gasAmount(%d)", MAXUINT64, toBalance, gasAmount)
	}

	return setBalance(bc.sdb, to, new(big.Int).SetUint64(toBalance+gasAmount))
}

// GetBalance get the balance of the address
func (bc *Blockchain) GetBalance(address address.Address) (uint64, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.getBalance(address)
}

// getBalance get the balance of the address
func (bc *Blockchain) getBalance(address address.Address) (uint64, error) {
	coAddr, err := address.NewCommonAddr()
	if err != nil {
		return 0, err
	}
	balance := bc.sdb.GetBalance(coAddr)
	return balance.Uint64(), nil
}

// GetNonce get the nonce of the address
func (bc *Blockchain) GetNonce(address address.Address) (uint64, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	return bc.getNonce(address)
}

// getNonce get the nonce of the address
func (bc *Blockchain) getNonce(address address.Address) (uint64, error) {
	var initNonce uint64 = 1

	coAddr, err := address.NewCommonAddr()
	if err != nil {
		return 0, err
	}

	n := bc.sdb.GetNonce(coAddr)
	if n == 0 {
		return initNonce, nil
	}

	return n, nil
}

func getSnapRootLock(db store.DB) (common.Hash, error) {
	var mu sync.RWMutex
	mu.RLock()
	defer mu.RUnlock()

	sr, err := db.Get(SnapRootKey)
	if err == store.NotExist {
		return common.Hash{}, nil
	} else if err != nil {
		return common.Hash{}, err
	}
	return common.BytesToHash(sr), nil
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

// factCommit writes the state to the underlying in-memory trie database
func (bc *Blockchain) FactCommit(deleteEmptyObjects bool) (common.Hash, error) {
	ha, err := bc.sdb.Commit(deleteEmptyObjects)
	if err != nil {
		logger.Error("stateDB commit error")
		return common.Hash{}, err
	}
	triDB := bc.sdb.Database().TrieDB()
	err = triDB.Commit(ha, true, nil)
	if err != nil {
		logger.Error("triDB commit error")
		return ha, err
	}
	return ha, err
}

// factCommit writes the state to the underlying in-memory trie database
func fakeCommit(sdb *state.StateDB, deleteEmptyObjects bool) (common.Hash, error) {
	sdbc := sdb.Copy()
	ha, err := sdbc.Commit(deleteEmptyObjects)
	if err != nil {
		logger.Error("stateDB commit error")
		return common.Hash{}, err
	}
	triDB := sdbc.Database().TrieDB()
	err = triDB.Commit(ha, true, nil)
	if err != nil {
		logger.Error("triDB commit error")
		return ha, err
	}

	return ha, err
}
