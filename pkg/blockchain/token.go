package blockchain

import (
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/contract/exec"
	"github.com/korthochain/korthochain/pkg/contract/parser"
)

// GetTokenBalance get the balance of tokens
func (bc *Blockchain) GetTokenBalance(addr address.Address, symbol []byte) (uint64, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	coAddr, err := addr.NewCommonAddr()
	if err != nil {
		return 0, err
	}

	balance, err := exec.TokenBalance(bc.sdb, string(symbol), coAddr.String())
	if err != nil {
		return 0, err
	}

	return balance, nil
}

// GetFrozenTokenBal get the frozen balance of tokens
func (bc *Blockchain) GetFrozenTokenBal(addr address.Address, symbol []byte) (uint64, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	coAddr, err := addr.NewCommonAddr()
	if err != nil {
		return 0, err
	}

	feeBalance, err := exec.TokenFreeze(bc.sdb, string(symbol), coAddr.String())
	if err != nil {
		return 0, err
	}
	return feeBalance, nil
}

// GetTokenRoot get token root
func (bc *Blockchain) GetTokenRoot(addr address.Address, script string) ([]byte, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	coAddr, err := addr.NewCommonAddr()
	if err != nil {
		return nil, err
	}

	dbTransaction := bc.db.NewTransaction()
	defer dbTransaction.Cancel()
	e, err := exec.New(dbTransaction, bc.sdb, parser.Parser([]byte(script)), coAddr.String())
	if err != nil {
		return nil, err
	}
	dbTransaction.Commit()

	return e.Root(), nil
}

// GetTokenDemic get token precision
func (bc *Blockchain) GetTokenDemic(symbol []byte) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	b, err := exec.Precision(bc.db, string(symbol))
	if err != nil {
		return 0, err
	}
	return b, nil
}
