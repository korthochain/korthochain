package evm

import (
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/korthochain/korthochain/pkg/blockchain"
	"github.com/korthochain/korthochain/pkg/config"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// Evm struct
type Evm struct {
	cfg *runtime.Config
	sdb *state.StateDB
	bc  *blockchain.Blockchain
}

func NewEvm(sdb *state.StateDB, bc *blockchain.Blockchain) *Evm {
	e := &Evm{cfg: new(runtime.Config), sdb: sdb, bc: bc}
	setDefaults(e.cfg)
	e.cfg.State = sdb
	return e
}

//Get evm dump
func (e *Evm) RawDump() state.Dump {
	if e.cfg.State != nil {
		//return e.cfg.State.RawDump(false, false, false)
		return e.cfg.State.RawDump(nil)
	}
	return state.Dump{}
}

//Create a contract
func (e *Evm) Create(code []byte, origin common.Address) ([]byte, common.Address, uint64, error) {
	if len(origin) > 0 {
		e.cfg.Origin = origin
	}
	return runtime.Create(code, e.cfg)
}

//Call contract
func (e *Evm) Call(contAddr common.Address, origin common.Address, inputCode []byte) ([]byte, uint64, error) {
	log.Printf("contract address:[%v],inputCode{%v},origin[%v]", contAddr, common.Bytes2Hex(inputCode), origin)
	getcode := e.cfg.State.GetCode(contAddr)
	if len(getcode) <= 0 {
		return nil, 0, fmt.Errorf("Call error:GetCode failed by contractaddress[%v]", contAddr)
	}

	e.cfg.State.SetCode(contAddr, e.cfg.State.GetCode(contAddr))
	e.cfg.Origin = origin

	return runtime.Call(contAddr, inputCode, e.cfg)
}

//Get contract bytecode
func (e *Evm) GetCode(contAddr common.Address) []byte {
	return e.cfg.State.GetCode(contAddr)
}

//Prepare hash into evm
func (e *Evm) Prepare(txhash, blhash common.Hash, txindex int) {
	e.cfg.State.Prepare(txhash, txindex)
}

//Add log
func (e *Evm) AddLog(lg *types.Log) {
	e.cfg.State.AddLog(lg)
}

//Get logs
func (e *Evm) GetLogs(cmhash common.Hash) []*types.Log {
	return e.cfg.State.GetLogs(cmhash, common.Hash{})
}

//Get logs
func (e *Evm) Logs() []*types.Log {
	return e.cfg.State.Logs()
}

//SetBlockInfo set block info into evm
func (e *Evm) SetBlockInfo(num uint64, miner string, tm uint64) {
	if num >= 0 {
		e.cfg.BlockNumber = new(big.Int).SetUint64(num)
	}
	if len(miner) > 0 {
		e.cfg.Coinbase = common.HexToAddress(miner)
	}
	if tm != 0 {
		e.cfg.Time = new(big.Int).SetUint64(tm)
	}
	//e.cfg.GasLimit = gasLimt
}

//Set value into evm
func (e *Evm) SetConfig(val, price *big.Int, limit uint64) {
	e.cfg.Value = val
	e.cfg.GasPrice = price
	e.cfg.GasLimit = limit
}

//Get evm Config
func (e *Evm) GetConfig() *runtime.Config {
	return e.cfg
}

//Add Balance
func (e *Evm) AddBalance(addr common.Address, amount *big.Int) {
	e.cfg.State.AddBalance(addr, amount)
}

//Sub Balance
func (e *Evm) SubBalance(addr common.Address, amount *big.Int) {
	e.cfg.State.SubBalance(addr, amount)
}

//Set Balance
func (e *Evm) SetBalance(addr common.Address, amount *big.Int) {
	e.cfg.State.SetBalance(addr, amount)
}

//Get Balance
func (e *Evm) GetBalance(addr common.Address) *big.Int {
	return e.cfg.State.GetBalance(addr)
}

//Get Nonce
func (e *Evm) GetNonce(addr common.Address) uint64 {
	return e.cfg.State.GetNonce(addr)
}

//Get Storage At address
func (e *Evm) GetStorageAt(addr common.Address, hash common.Hash) common.Hash {
	proof := e.cfg.State.GetState(addr, hash)
	return proof
}

//Get Snapshot
func (e *Evm) GetSnapshot() int {
	return e.cfg.State.Snapshot()
}

//Revert Snapshot to a position
func (e *Evm) RevertToSnapshot(sp int) {
	e.cfg.State.RevertToSnapshot(sp)

}

//sets defaults config
func setDefaults(cfg *runtime.Config) {
	var chainId *big.Int
	if config.GlobalCfg.BFTConfig != nil {
		chainId = big.NewInt(config.GlobalCfg.BFTConfig.ChainId)
	} else {
		chainId = big.NewInt(2559)
	}

	if cfg.ChainConfig == nil {
		cfg.ChainConfig = &params.ChainConfig{
			ChainID:             chainId,
			HomesteadBlock:      new(big.Int),
			DAOForkBlock:        new(big.Int),
			DAOForkSupport:      false,
			EIP150Block:         new(big.Int),
			EIP150Hash:          common.Hash{},
			EIP155Block:         new(big.Int),
			EIP158Block:         new(big.Int),
			ByzantiumBlock:      new(big.Int),
			ConstantinopleBlock: new(big.Int),
			PetersburgBlock:     new(big.Int),
			IstanbulBlock:       new(big.Int),
			MuirGlacierBlock:    new(big.Int),
			//YoloV3Block:         nil,
		}
	}

	if cfg.Difficulty == nil {
		cfg.Difficulty = new(big.Int)
	}
	if cfg.Time == nil {
		cfg.Time = big.NewInt(time.Now().Unix())
	}
	if cfg.GasLimit == 0 {
		//cfg.GasLimit = math.MaxUint64
		if config.GlobalCfg.BFTConfig != nil {
			cfg.GasLimit = config.GlobalCfg.BFTConfig.GasLimit
		} else {
			cfg.GasLimit = 10000000
		}
	}
	if cfg.GasPrice == nil {
		if config.GlobalCfg.BFTConfig != nil {
			cfg.GasPrice = big.NewInt(int64(config.GlobalCfg.BFTConfig.GasPrice))
		} else {
			cfg.GasPrice = big.NewInt(21000)
		}

	}
	if cfg.Value == nil {
		cfg.Value = new(big.Int)
	}
	if cfg.BlockNumber == nil {
		cfg.BlockNumber = new(big.Int)
	}
	if cfg.GetHashFn == nil {
		cfg.GetHashFn = func(n uint64) common.Hash {
			return common.BytesToHash(crypto.Keccak256([]byte(new(big.Int).SetUint64(n).String())))
		}
	}
}
