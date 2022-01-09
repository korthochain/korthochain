// It is realized by setting two frozen addresses
// from: total frozen quota
// from+to: frozen amount of designated notary

package blockchain

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/korthochain/korthochain/pkg/address"
)

// setFreezeAccount update and unfreeze account information
// code only has 0 or 1
func setFreezeAccount(bc *Blockchain, from address.Address, to address.Address, freezeBal uint64, gasUsed uint64, code int) error {
	var setSingFree, setAllFree uint64

	singleFree, err := bc.getSingleFreezeBalance(from, to)
	if err != nil {
		return err
	}

	allFree, err := bc.getAllFreezeBalance(from)
	if err != nil {
		return err
	}

	// code 1 means frozen, otherwise means thawing
	if code == 1 {
		{
			availableBal, err := bc.getAvailableBalance(from)
			if err != nil {
				return err
			}
			if freezeBal > availableBal+gasUsed {
				return fmt.Errorf("freezeBal is too large,freezeBal(%d) > availableBal(%d)", freezeBal, availableBal)
			}
		}

		setSingFree = singleFree + freezeBal
		setAllFree = allFree + freezeBal
		if MAXUINT64-singleFree-gasUsed < freezeBal {
			return fmt.Errorf("freezeBal is too large,max maxuint64(%d)-singleFree(%d) < freezeBal(%d)", MAXUINT64, singleFree, freezeBal)
		}
		if MAXUINT64-allFree-gasUsed < freezeBal {
			return fmt.Errorf("freezeBal is too large,max maxuint64(%d)-allFree(%d) < freezeBal(%d)", MAXUINT64, allFree, freezeBal)
		}
	} else {
		{
			availableBal, err := bc.getAvailableBalance(from)
			if err != nil {
				return err
			}
			if gasUsed > availableBal {
				return fmt.Errorf("availableBal balance insufficient, gasUsed(%d) > availableBal(%d)", gasUsed, availableBal)
			}
		}

		setSingFree = singleFree - freezeBal
		setAllFree = allFree - freezeBal
		if singleFree < freezeBal {
			return fmt.Errorf("unfreezing is too large,singleFree(%d) < freezeBal(%d)", singleFree, freezeBal)
		}
		if allFree < freezeBal {
			return fmt.Errorf("unfreezing is too large,allFree(%d) < freezeBal(%d)", allFree, freezeBal)
		}
	}

	setSingleFreezeBalance(bc.sdb, from, to, new(big.Int).SetUint64(setSingFree))
	setAllFreezeBalance(bc.sdb, from, new(big.Int).SetUint64(setAllFree))

	fromBalance, err := bc.getBalance(from)
	if MAXUINT64-gasUsed < fromBalance {
		return fmt.Errorf("freezeBal is too large,max maxuint64(%d)-gasUsed(%d) < fromBalance(%d)", MAXUINT64, gasUsed, fromBalance)
	}

	if err := setBalance(bc.sdb, from, new(big.Int).SetUint64(fromBalance-gasUsed)); err != nil {
		return fmt.Errorf("setBalance error")
	}

	return nil
}

// setSingleFreezeBalance set the quota frozen for a single address
// from: Frozen address
// to: Notary Public
func setSingleFreezeBalance(s *state.StateDB, from address.Address, to address.Address, freezeBal *big.Int) error {
	coFromAddr, err := from.NewCommonAddr()
	if err != nil {
		return err
	}
	fromKey := commonAddrToStoreAddr(coFromAddr, FreezeKey)
	s.SetBalance(commonAddrToStoreAddr(fromKey, to.Bytes()), freezeBal)
	return nil
}

// setAllFreezeBalance set the total frozen quota of an address
// from: Frozen address
func setAllFreezeBalance(s *state.StateDB, from address.Address, freezeBal *big.Int) error {
	coFromAddr, err := from.NewCommonAddr()
	if err != nil {
		return err
	}
	s.SetBalance(commonAddrToStoreAddr(coFromAddr, FreezeKey), freezeBal)
	return nil
}

// GetAllFreezeBalance query all frozen quotas of an address
func (bc *Blockchain) GetAllFreezeBalance(address address.Address) (uint64, error) {
	//	bc.mu.RLock()
	//	defer bc.mu.RUnlock()
	bc.mu.Lock()
	defer bc.mu.Unlock()

	return bc.getAllFreezeBalance(address)
}

func (bc *Blockchain) getAllFreezeBalance(address address.Address) (uint64, error) {
	addr, err := address.NewCommonAddr()
	if err != nil {
		return 0, err
	}
	freezeBal := bc.sdb.GetBalance(commonAddrToStoreAddr(addr, FreezeKey))
	return freezeBal.Uint64(), nil
}

// GetSingleFreezeBalance check an address and specify the frozen amount of the notary
// from: Frozen address. to: Notary Public
func (bc *Blockchain) GetSingleFreezeBalance(address address.Address, to address.Address) (uint64, error) {
	//	bc.mu.RLock()
	//	defer bc.mu.RUnlock()
	bc.mu.Lock()
	defer bc.mu.Unlock()

	return bc.getSingleFreezeBalance(address, to)
}

func (bc *Blockchain) getSingleFreezeBalance(address address.Address, to address.Address) (uint64, error) {
	addr, err := address.NewCommonAddr()
	if err != nil {
		return 0, err
	}
	fromKey := commonAddrToStoreAddr(addr, FreezeKey)
	freezeBal := bc.sdb.GetBalance(commonAddrToStoreAddr(fromKey, to.Bytes()))
	return freezeBal.Uint64(), nil
}
