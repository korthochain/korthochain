package blockchain

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/storage/miscellaneous"
	"github.com/korthochain/korthochain/pkg/storage/store"
	"github.com/korthochain/korthochain/pkg/transaction"
	"go.uber.org/zap"
)

var (
	PledgeKey      = []byte("PledgeInfo")
	TotalPledgeKey = []byte("TotalPledgeKey")
)

const (
	DayPledge      int = 100
	PledgeCycle    int = 120
	DayMinedBlocks int = 8640
	CycleMax           = PledgeCycle * DayMinedBlocks
)

type NodeKeyPledge struct {
	node             string
	startCycleHeight uint64
}
type NodeValuePledge struct {
	releaseValue       uint64 //the release value of each time
	startReleaseHeight uint64 //the block start to release node pledge
	stopReleaseHeight  uint64 //the block end to release
}

//add pledge into pldege pool
func setTotalPledge(sdb *state.StateDB, addr, pledge []byte) error {
	ak := miscellaneous.EMapKey(TotalPledgeKey, addr)
	addrress := miscellaneous.BytesSha1Address(ak)

	bigPlg, err := miscellaneous.D64func(pledge)
	if err != nil {
		logger.Error("miscellaneous D64func", zap.Error(err))
		return err
	}
	sdb.SetBalance(addrress, new(big.Int).SetUint64(bigPlg))
	return nil
}

// Get address Total Pledge
func (bc *Blockchain) GetTotalPledge(address []byte) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return getTotalPledge(bc.sdb, address)
}

// Get address Total Pledge
func getTotalPledge(sdb *state.StateDB, address []byte) (uint64, error) {
	ak := miscellaneous.EMapKey(TotalPledgeKey, address)
	addr := miscellaneous.BytesSha1Address(ak)

	ttpBytes := sdb.GetBalance(addr)
	return ttpBytes.Uint64(), nil
}

//handle pledge transaction
func (bc *Blockchain) handlePledgeTransaction(block *block.Block, DBTransaction store.Transaction, tx *transaction.Transaction) error {
	//get availableBalance
	// if availableBalance < tx.Amount {
	// 	return fmt.Errorf("The pledge balance is insufficient available[%v] < amount[%v]", availableBalance, tx.Amount)
	// }
	totPledge, err := getTotalPledge(bc.sdb, tx.From.Bytes())
	if err != nil {
		return err
	}
	//init node first pledge info
	if totPledge == 0 {
		startCycleHeight := block.Height
		startReleaseHeight := startCycleHeight + 1 + uint64(CycleMax)
		stopReleaseHeight := startReleaseHeight - 1 + uint64(CycleMax)

		var nkp NodeKeyPledge
		nkp.node = tx.From.String()
		nkp.startCycleHeight = startCycleHeight

		var nvp NodeValuePledge
		nvp.releaseValue = 0
		nvp.startReleaseHeight = startReleaseHeight
		nvp.stopReleaseHeight = stopReleaseHeight

		err := setPledgeInfo(DBTransaction, &nkp, &nvp)
		if err != nil {
			return err
		}
	}

	totPledge += tx.Amount

	return setTotalPledge(bc.sdb, tx.From.Bytes(), miscellaneous.E64func(totPledge))
}

//set pledge info
func setPledgeInfo(DBTransaction store.Transaction, nodeKP *NodeKeyPledge, nodeVP *NodeValuePledge) error {
	keyByte, err := json.Marshal(nodeKP)
	if err != nil {
		return err
	}

	valueByte, err := json.Marshal(nodeVP)
	if err != nil {
		return err
	}
	return DBTransaction.Mset(PledgeKey, keyByte, valueByte)
}

//get pledge keys info
func getPledgeKVsInfo(DBTransaction store.Transaction, prefix []byte) ([][]byte, [][]byte, error) {
	return DBTransaction.Mkvs(prefix)
}

//get pledge value by key
func getPledgeValueInfo(DBTransaction store.Transaction, nodeKP *NodeKeyPledge) ([]byte, error) {
	keyByte, err := json.Marshal(nodeKP)
	if err != nil {
		return nil, err
	}
	return DBTransaction.Mget(PledgeKey, keyByte)
}

//release pledge if need
func (bc *Blockchain) releasePlege(DBTransaction store.Transaction, block *block.Block) error {
	ks, vs, err := getPledgeKVsInfo(DBTransaction, PledgeKey)
	if err != nil {
		return err
	}

	var pledgeKs []*NodeKeyPledge
	for _, k := range ks {
		err = json.Unmarshal(k, &pledgeKs)
		if err != nil {
			logger.Error("json Unmarshal NodeKeyPledge error", zap.Error(err))
			return err
		}
	}

	var pledgeVs []*NodeValuePledge
	for _, v := range vs {
		err = json.Unmarshal(v, &pledgeVs)
		if err != nil {
			logger.Error("json Unmarshal NodeValuePledge error", zap.Error(err))
			return err
		}
	}

	for i, k := range pledgeKs {
		v := pledgeVs[i]
		addr, err := address.NewAddrFromString(k.node)
		if err != nil {
			logger.Error("NewAddrFromString error", zap.Error(err))
			continue
		}
		totPledge, err := getTotalPledge(bc.sdb, addr.Bytes())
		if err != nil {
			logger.Error("getTotalPledge error", zap.Error(err))
			continue
		}
		if totPledge > 0 {
			if block.Height > k.startCycleHeight {
				if block.Height >= v.startReleaseHeight && block.Height <= v.stopReleaseHeight {
					releaseValue := v.releaseValue
					if block.Height == v.startReleaseHeight && releaseValue == 0 {
						v.releaseValue = totPledge / uint64(CycleMax)
						err := setPledgeInfo(DBTransaction, k, v) //update releaseValue
						if err != nil {
							logger.Error("setPledgeInfo error", zap.Error(err))
							return err
						}
						releaseValue = v.releaseValue
					}

					//release
					if totPledge >= releaseValue {
						totPledge -= releaseValue
					}
					if totPledge < releaseValue && totPledge > 0 {
						totPledge -= totPledge
					}
					err := setTotalPledge(bc.sdb, []byte(k.node), miscellaneous.E64func(totPledge))
					if err != nil {
						logger.Error("setTotalPledge error", zap.Error(err))
						return err
					}

					//update next release cycle.
					if block.Height == v.stopReleaseHeight {
						keyByte, err := json.Marshal(k)
						if err != nil {
							return err
						}
						DBTransaction.Mdel(PledgeKey, keyByte) //delete last cycle info

						k.startCycleHeight = block.Height
						v.startReleaseHeight = v.stopReleaseHeight + 1
						v.stopReleaseHeight = v.startReleaseHeight - 1 + uint64(CycleMax)
						releaseValue = 0
						err = setPledgeInfo(DBTransaction, k, v)
						if err != nil {
							logger.Error("setPledgeInfo error", zap.Error(err))
							return err
						}
					}
				}
			}
		}
	}
	return nil
}
