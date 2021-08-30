package blockchain

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/storage/miscellaneous"
	"go.uber.org/zap"

	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/storage/store"
)

const (
	DIVID    = 7  //dividend
	DIVIDEND = 10 //divid
)

var (
	MinedKey      = []byte("MinedKey")
	TotalMinedKey = []byte("TotalMinedKey")
)

//PledgeKeyInfo struct
type DistriKeyInfo struct {
	blockMiner  string //current block miner
	minedHeight uint64 //mined block height
}

//PledgeKeyInfo struct
type DistriValueInfo struct {
	releaseValue       uint64 //the release value of each time
	startReleaseHeight uint64 //the block start to release mined pledge
	stopReleaseHeight  uint64 //the block end to release
}

//set total mined pledge
func setTotalMined(s *state.StateDB, addr, minedVal []byte) error {
	ak := miscellaneous.EMapKey(TotalMinedKey, addr)
	addrress := miscellaneous.BytesSha1Address(ak)

	bigMind, err := miscellaneous.D64func(minedVal)
	if err != nil {
		logger.Error("miscellaneous D64func", zap.Error(err))
		return err
	}
	s.SetBalance(addrress, new(big.Int).SetUint64(bigMind))
	return nil
}

// Get Total Mined pledge by address
func (bc *Blockchain) GetTotalMined(address address.Address) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return getTotalMined(bc.sdb, address.Bytes())
}

func getTotalMined(s *state.StateDB, address []byte) (uint64, error) {
	ak := miscellaneous.EMapKey(TotalMinedKey, address)
	addr := miscellaneous.BytesSha1Address(ak)

	ttmBytes := s.GetBalance(addr)
	return ttmBytes.Uint64(), nil
}

//set Distribution info
func setDistriInfo(DBTransaction store.Transaction, dki *DistriKeyInfo, dvi *DistriValueInfo) error {
	keyByte, err := json.Marshal(dki)
	if err != nil {
		return err
	}

	valueByte, err := json.Marshal(dvi)
	if err != nil {
		return err
	}
	return DBTransaction.Mset(MinedKey, keyByte, valueByte)
}

//pledge 70% mined
func (bc *Blockchain) handleMinedPledge(DBTransaction store.Transaction, block *block.Block, to address.Address, amount uint64) error {
	totMinedPledge, err := getTotalMined(bc.sdb, to.Bytes())
	if err != nil {
		return err
	}

	if block.Miner == to {
		minedPledge := amount * DIVID / DIVIDEND
		totMinedPledge += minedPledge
		err := setTotalMined(bc.sdb, to.Bytes(), miscellaneous.E64func(totMinedPledge))
		if err != nil {
			return err
		}

		var dki DistriKeyInfo
		dki.blockMiner = block.Miner.String()
		dki.minedHeight = block.Height

		var dvi DistriValueInfo
		dvi.releaseValue = minedPledge / uint64(CycleMax)
		dvi.startReleaseHeight = dki.minedHeight + uint64(CycleMax)
		dvi.stopReleaseHeight = dki.minedHeight + uint64(2*CycleMax)

		err = setDistriInfo(DBTransaction, &dki, &dvi)
		if err != nil {
			return err
		}
	}
	return nil
}

//release mined pledge
func (bc *Blockchain) releaseMined(DBTransaction store.Transaction, block *block.Block) error {
	ks, vs, err := getPledgeKVsInfo(DBTransaction, MinedKey)
	if err != nil {
		return err
	}

	var mdKs []*DistriKeyInfo
	for _, k := range ks {
		err = json.Unmarshal(k, &mdKs)
		if err != nil {
			logger.Error("json Unmarshal DistriKeyInfo error", zap.Error(err))
			return err
		}
	}

	var mdVs []*DistriValueInfo
	for _, v := range vs {
		err = json.Unmarshal(v, &mdVs)
		if err != nil {
			logger.Error("json Unmarshal DistriValueInfo error", zap.Error(err))
			return err
		}
	}

	for i, k := range mdKs {
		if block.Height > k.minedHeight {
			addr, err := address.NewAddrFromString(k.blockMiner)
			if err != nil {
				logger.Error("json Unmarshal DistriValueInfo error", zap.Error(err))
				return err
			}
			totMinedPledge, err := getTotalMined(bc.sdb, addr.Bytes())
			if err != nil {
				return err
			}
			v := mdVs[i]
			if totMinedPledge > 0 {
				if block.Height >= v.startReleaseHeight && block.Height <= v.stopReleaseHeight {
					if totMinedPledge >= v.releaseValue {
						totMinedPledge -= v.releaseValue
					} else if totMinedPledge < v.releaseValue {
						totMinedPledge = 0
					}
					err := setTotalMined(bc.sdb, []byte(k.blockMiner), miscellaneous.E64func(totMinedPledge))
					if err != nil {
						logger.Error("setTotalMined error", zap.Error(err))
						return err
					}

					if block.Height == v.stopReleaseHeight {
						keyByte, err := json.Marshal(k)
						if err != nil {
							logger.Error("json Marshal error", zap.Error(err))
							return err
						}
						DBTransaction.Mdel(MinedKey, keyByte)
					}
				}
			}
		}
	}
	return nil
}
