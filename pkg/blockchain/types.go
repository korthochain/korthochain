package blockchain

import (
	"crypto/sha1"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/korthochain/korthochain/pkg/contract/evm"
	"github.com/korthochain/korthochain/pkg/storage/store"
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

	BindingKey     = []byte("binding")
	CREATECONTRACT = "create"
	CALLCONTRACT   = "call"
)

// Blockchain blockchain data structure
type Blockchain struct {
	mu       sync.RWMutex
	db       store.DB
	sdb      *state.StateDB
	evm      *evm.Evm
	ChainCfg *ChainConfig
}

var ETHDECIMAL uint64 = 10000000

const MAXGASLIMIT uint64 = 10000000000000
const MINGASLIMIT uint64 = 2100000

// TxIndex transaction data index structure
type TxIndex struct {
	Height uint64
	Index  uint64
}

const (
	MinPledgeAmount uint64 = 10000000000000
	MaxPledgeAmount uint64 = 1200000000000000
)

func Check0x(input string) string {
	if len(input) > 2 {
		if input[:2] == "0x" {
			input = input[2:]
		}
	}
	return input
}

func commonAddrToStoreAddr(caddr common.Address, prefix []byte) common.Address {
	hash := sha1.Sum(append(prefix, caddr.Bytes()...))
	caddr.SetBytes(hash[:])
	return caddr
}

func CommonAddrToStoreAddr(caddr common.Address, prefix []byte) common.Address {
	hash := sha1.Sum(append(prefix, caddr.Bytes()...))
	caddr.SetBytes(hash[:])
	return caddr
}

func Uint64ToBigInt(v uint64) *big.Int {
	return new(big.Int).SetUint64(v)
}

func GetMinerAmount(height uint64) uint64 {
	const newChainHeight uint64 = InitHeight
	const ktoTotal uint64 = 8848000000000000000
	const oldPhase1 uint64 = 31536000 * 49460000000
	const oldPhase2 uint64 = (newChainHeight - 31536000) * 8 / 10 * 49460000000
	const newChainBalance uint64 = 1000000000000000000 + oldPhase1 + oldPhase2
	var newChantotal uint64 = ktoTotal - newChainBalance
	if height < newChainHeight {
		err := fmt.Errorf("error height < newChainHeight")
		panic(err)
	}

	height = height - newChainHeight
	var total uint64 = newChantotal / 3153600 / 10
	x := height / 3153600
	for i := 0; uint64(i) < x; i++ {
		newChantotal = newChantotal - (3153600 * total)
		total = newChantotal / 3153600 / 10
	}
	return total
}
