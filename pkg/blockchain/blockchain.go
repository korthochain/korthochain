package blockchain

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/dgraph-io/badger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"korthochain/pkg/storage/miscellaneous"
	"korthochain/pkg/storage/store"
	"korthochain/pkg/storage/store/bg"
	"korthochain/pkg/storage/store/bg/bgdb"
	"math/big"
	"sync"
	"unsafe"
)

const (
	// BlockchainDBName blockchain Database name
	BlockchainDBName = "blockchain.db"
	// ContractDBName contract database name
	ContractDBName = "contract.db"

	TRS  = "a9059cbb"
	TRSF = "23b872dd"
	APPR = "095ea7b3"

	DECI = "313ce567"
)

const (
	MAXUINT64 = ^uint64(0)
)

var (
	//SnapRoot key to store snaproot in database
	SnapRoot = []byte("snapRoot")
	// HeightKey key to store height in database
	HeightKey = []byte("height")
	// NonceKey store the map name of nonce
	NonceKey = []byte("nonce")
	//FreezeKey store the map name of freeze balance
	FreezeKey = []byte("freeze")
	//BheightKey = []byte("bheight")
	BindingKey = []byte("binding")
)

var (
	// AddrListPrefix addrlistprefix is the prefix of the list name. Each addreess maintains a list in the database
	AddrListPrefix = []byte("addr")
	// HeightPrefix prefix of block height key
	HeightPrefix = []byte("blockheight")
	// TxListName name of the transaction list
	TxListName = []byte("txlist")
)

// Blockchain blockchain data structure
type Blockchain struct {
	mu  sync.RWMutex
	db  store.DB
	sdb *state.StateDB
}

type TXindex struct {
	Height uint64
	Index  uint64
}

// New create blockchain object
func New(db *badger.DB) (*Blockchain, error) {
	bgs := bg.New(db)
	cdb := bgdb.NewBadgerDatabase(bgs)
	sdb := state.NewDatabase(cdb)
	//root := common.Hash{}
	root := getSnapRoot(bgs)
	stdb, err := state.New(root, sdb, nil)
	if err != nil {
		logger.Error("failed to new state.")
	}

	bc := &Blockchain{db: bgs, sdb: stdb}
	return bc, nil
}

//setNonce set address nonce
func setNonce(s *state.StateDB, addr, nonce []byte) {
	a := BytesSha1Address(addr)
	n, err := miscellaneous.D64func(nonce)
	if err != nil {
		logger.Error("error from miscellaneous.D64func")
	}
	s.SetNonce(a, n)
}

//setBalance set address balance
func setBalance(s *state.StateDB, addr, balance []byte) {
	a := BytesSha1Address(addr)
	balanceU, err := miscellaneous.D64func(balance)
	if err != nil {
		logger.Error("error from miscellaneous.D64func")
	}

	s.SetBalance(a, new(big.Int).SetUint64(balanceU))
}

//setFreezeBalance set address freeze balance
func setFreezeBalance(s *state.StateDB, addr, freezeBal []byte) {
	ak := eMapKey(FreezeKey, addr)
	a := BytesSha1Address(ak)
	freezeBalU, err := miscellaneous.D64func(freezeBal)
	if err != nil {
		logger.Error("error from miscellaneous.D64func")
	}

	s.SetBalance(a, new(big.Int).SetUint64(freezeBalU))
}

func setAccount(sdb *state.StateDB, tx *transaction.Transaction) error {
	from, to := tx.From.Bytes(), tx.To.Bytes()

	fromCA := BytesSha1Address(from)
	fromBalBig := sdb.GetBalance(fromCA)
	fromBalance := fromBalBig.Uint64()
	if tx.IsTokenTransaction() {
		if fromBalance < tx.Amount+tx.Fee {
			return fmt.Errorf("not sufficient funds,hash:%s,from balance(%d) < amount(%d)+fee(%d)",
				hex.EncodeToString(tx.Hash), fromBalance, tx.Amount, tx.Fee)
		}
		fromBalance -= tx.Amount + tx.Fee
	} else {
		if fromBalance < tx.Amount {
			return fmt.Errorf("not sufficient funds,hash:%s,from balance(%d) < amount(%d)",
				hex.EncodeToString(tx.Hash), fromBalance, tx.Amount)
		}
		fromBalance -= tx.Amount
	}

	toCA := BytesSha1Address(to)
	tobalance := sdb.GetBalance(toCA)
	toBalance := tobalance.Uint64()
	if MAXUINT64-toBalance < tx.Amount {
		return fmt.Errorf("amount is too large,hash:%s,max int64(%d)-balance(%d) < amount(%d)", tx.Hash, MAXUINT64, toBalance, tx.Amount)
	}
	toBalance += tx.Amount

	Frombytes := miscellaneous.E64func(fromBalance)
	Tobytes := miscellaneous.E64func(toBalance)

	setBalance(sdb, from, Frombytes)
	setBalance(sdb, to, Tobytes)

	return nil
}

func setToAccount(sdb *state.StateDB, tx *transaction.Transaction) error {
	to := tx.To.Bytes()
	toCA := BytesSha1Address(to)

	toBalanceBig := sdb.GetBalance(toCA)
	balance := toBalanceBig.Uint64()

	if MAXUINT64-tx.Amount < balance {
		return fmt.Errorf("not sufficient funds")
	}
	newBalanceBytes := miscellaneous.E64func(balance + tx.Amount)
	setBalance(sdb, tx.To.Bytes(), newBalanceBytes)

	return nil
}

func addBalance(s *state.StateDB, addr, balance []byte) {
	a := BytesSha1Address(addr)
	balanceU, err := miscellaneous.D64func(balance)
	if err != nil {
		logger.Error("error from miscellaneous.D64func")
	}

	s.AddBalance(a, new(big.Int).SetUint64(balanceU))
}

func subBalance(s *state.StateDB, addr, balance []byte) {
	a := BytesSha1Address(addr)
	balanceU, err := miscellaneous.D64func(balance)
	if err != nil {
		logger.Error("error from miscellaneous.D64func")
	}

	s.SubBalance(a, new(big.Int).SetUint64(balanceU))
}

// GetBalance Get the balance of the address
func (bc *Blockchain) GetBalance(address []byte) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getBalance(address)
}

func (bc *Blockchain) getBalance(address []byte) (uint64, error) {
	addr := BytesSha1Address(address)
	balanceBig := bc.sdb.GetBalance(addr)
	//	return new(big.Int).Set(balanceBig).Uint64()
	return balanceBig.Uint64(), nil
}

// GetFreezeBalance Get the freeze balance of the address
func (bc *Blockchain) GetFreezeBalance(address []byte) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getFreezeBalance(address)
}

func (bc *Blockchain) getFreezeBalance(address []byte) (uint64, error) {
	ak := eMapKey(FreezeKey, address)
	freezeAddr := BytesSha1Address(ak)

	freezeBalBytes := bc.sdb.GetBalance(freezeAddr)
	return freezeBalBytes.Uint64(), nil
}

//GetNonce Get the nonce of the address
func (bc *Blockchain) GetNonce(address []byte) (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getNonce(address)
}

func (bc *Blockchain) getNonce(address []byte) (uint64, error) {
	addr := BytesSha1Address(address)
	//n := bc.sdb.GetNonce(common.BytesToAddress(address[:]))
	n := bc.sdb.GetNonce(addr)
	return n, nil
}

//getSnapRoot Get the SnapRoot of the DB
func getSnapRoot(db store.DB) common.Hash {
	sr, err := db.Get(SnapRoot)
	if err == store.NotExist {
		return common.Hash{}
	} else if err != nil {
		return common.Hash{}
	}
	return common.BytesToHash(sr)
}

//factCommit writes the state to the underlying in-memory trie database
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

// string transformation ytes
func str2bytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

// []byte transformation string
func bytes2str(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// []byte transformation Address
func BytesSha1Address(addr []byte) common.Address {
	addrString := bytes2str(addr)
	addrHAS1 := SHA1(addrString)
	a := common.HexToAddress(addrHAS1)
	return a
}

func SHA1(s string) string {
	o := sha1.New()
	o.Write([]byte(s))
	//o.Write(str2bytes(s))
	return hex.EncodeToString(o.Sum(nil))
}

func eMapKey(m, k []byte) []byte {
	buf := []byte{}
	buf = append([]byte{'m'}, miscellaneous.E32func(uint32(len(m)))...)
	buf = append(buf, m...)
	buf = append(buf, byte('+'))
	buf = append(buf, k...)
	return buf
}
