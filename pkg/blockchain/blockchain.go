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
	// BlockchainDBName blockchain数据库名称
	BlockchainDBName = "blockchain.db"
	// ContractDBName 合约数据库名称
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
	SnapRoot = []byte("snapRoot")
	// HeightKey 数据库中存储高度的键
	HeightKey = []byte("height")
	// NonceKey 存储nonce的map名
	NonceKey = []byte("nonce")
	//FreezeKey 冻结金额的map名
	FreezeKey = []byte("freeze")

	//BheightKey = []byte("bheight")
	BindingKey = []byte("binding")

	dKtoPrefix    = "dk"
	pckPrefix     = "pck"
	PckTotalName  = "pcktotal"
	DKtoTotalName = "dktototal"
	KTOAddress    = "ktoaddress"
	ETHAddress    = "ethaddress"

	ETHDECIMAL uint64 = 10000000
)

var (
	// AddrListPrefix 每个addreess在在数据库中都维护一个列表，AddrListPrefix是列表名的前缀
	AddrListPrefix = []byte("addr")
	// HeightPrefix 块高key的前缀
	HeightPrefix = []byte("blockheight")
	// TxListName 交易列表的名字
	TxListName = []byte("txlist")
)

// Blockchain 区块链数据结构
type Blockchain struct {
	mu sync.RWMutex
	db store.DB
	//	cdb         store.DB
	//	orderEngine *order.Engine
	//	evm         *evm.Evm

	sdb *state.StateDB
}

type TXindex struct {
	Height uint64
	Index  uint64
}

// New 创建区块链对象
/*
func New() (*Blockchain, error) {
	bgs := bg.New("blockchain.db")
	bgc := bg.New("contract.db")
	exdb := bg.New("exchange.db")

	ev, err := evm.NewEvm()
	if err != nil {
		//logger.Error("failed to new evm.")
		return nil, err
	}

	bc := &Blockchain{db: bgs, cdb: bgc, orderEngine: order.New(nil, exdb), evm: ev}
	return bc, nil
}
*/

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
	//a := common.BytesToAddress(addr[:])
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

//delete block
/*
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

		CDBTransaction := bc.cdb.NewTransaction()

		for i, tx := range block.Transactions {
			if tx.IsCoinBaseTransaction() {
				if err = deleteTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.Uint64("amount", tx.Amount))
					return err
				}

			} else if tx.IsFreezeTransaction() || tx.IsUnfreezeTransaction() {
				if err := deleteTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
					return err
				}

				if err := deleteTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
					return err
				}

			} else {
				//modify
				if tx.IsTokenTransaction() {
					spilt := strings.Split(tx.Script, "\"")
					if spilt[0] == "transfer " {
						script := fmt.Sprintf("transfer \"%s\" %s \"%s\"", spilt[1], spilt[2], tx.From.String())

						sc := parser.Parser([]byte(script))
						e, err := exec.New(CDBTransaction, sc, tx.To.String())
						if err != nil {
							logger.Error("Failed to new exec", zap.String("script", script),
								zap.String("from address", tx.To.String()))
							tx.Script += " InsufficientBalance"
						}

						if e != nil {
							if err = e.Flush(CDBTransaction); err != nil {
								logger.Error("Failed to flush exec", zap.String("script", tx.Script),
									zap.String("from address", tx.From.String()))
								tx.Script += " ExecutionFailed"
							}
						}
					}
					if err = delMinerFee(DBTransaction, block.Miner.Bytes(), tx.Fee); err != nil {
						logger.Error("Failed to set fee", zap.Error(err), zap.String("script", tx.Script),
							zap.String("from address", tx.From.String()), zap.Uint64("fee", tx.Fee))
						return err
					}
				}

				if err := deleteTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
					return err
				}

				if err := deleteTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(i)); err != nil {
					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
					return err
				}

			}
		}
		//========

		//高度->哈希
		hash := block.Hash
		if err = DBTransaction.Del(append(HeightPrefix, miscellaneous.E64func(block.Height)...)); err != nil {
			logger.Error("Failed to Del height and hash", zap.Error(err))
			return err
		}

		//哈希-> 块
		if err = DBTransaction.Del(hash); err != nil {
			logger.Error("Failed to Del block", zap.Error(err))
			return err
		}
		// last
		DBTransaction.Set(HeightKey, miscellaneous.E64func(dH-1))
		if err := DBTransaction.Commit(); err != nil {
			logger.Error("DBTransaction Commit err", zap.Error(err))
			return err
		}
		DBTransaction.Cancel()

		if err := CDBTransaction.Commit(); err != nil {
			logger.Error("DBTransaction Commit err", zap.Error(err))
			return err
		}
		CDBTransaction.Cancel()
	}

	logger.Info("End delete")
	return nil
}
*/

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
	//o.Write([]byte(s))
	o.Write(str2bytes(s))
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

//---------------------------------------------------------
//
//func (bc *Blockchain) SetDexAddr(dexAddr string) error {
//	addr, err := types.StringToAddress(dexAddr)
//	if err != nil {
//		return err
//	}
//	bc.orderEngine.Addr = (*addr).ToPublicKey()
//	return nil
//}
//
//// GetBlockchain 获取blockchain对象
////func GetBlockchain() *Blockchain {
////	return &Blockchain{db: bg.New(BlockchainDBName), cdb: bg.New(ContractDBName)}
////}
//
//func GetBlockchain(db *badger.DB) *Blockchain {
//	return &Blockchain{db: bg.New(db), cdb: bg.New(db)}
//}
//
//// NewBlock 通过输入的交易，新建block，minaddr,Ds,Cm,QTJ分别是矿工，社区，技术和趣淘鲸的地址
//func (bc *Blockchain) NewBlock(txs []*transaction.Transaction, minaddr, Ds, Cm, QTJ types.Address) (*block.Block, error) {
//	logger.Info("start to new block")
//	var height, prevHeight uint64
//	var prevHash []byte
//	prevHeight, err := bc.GetHeight()
//	if err != nil {
//		logger.Error("failed to get height", zap.Error(err))
//		return nil, err
//	}
//
//	height = prevHeight + 1
//	if height > 1 {
//		prevHash, err = bc.GetHash(prevHeight)
//		if err != nil {
//			logger.Error("failed to get hash", zap.Error(err), zap.Uint64("previous height", prevHeight))
//			return nil, err
//		}
//	} else {
//		prevHash = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
//	}
//
//	//出币分配
//	txs = Distr(txs, minaddr, Ds, Cm, QTJ, height)
//
//	/* 	//锁仓收益分红规则，社区分配收益到链上执行，每天执行一次
//	   	//分配锁仓收益
//	   	if height%24*60*60 == 0 {
//	   		err := bc.ShareOutBouns(txs)
//	   		if err != nil {
//	   			logger.Error("failed from shareoutbouns to do :", zap.Error(err))
//	   		}
//	   	} */
//	//生成默克尔根,如果没有交易的话，调用GetMtHash会painc
//	txBytesList := make([][]byte, 0, len(txs))
//	for _, tx := range txs {
//		tx.BlockNumber = height
//		txBytesList = append(txBytesList, tx.Serialize())
//	}
//	tree := merkle.New(sha256.New(), txBytesList)
//	root := tree.GetMtHash()
//
//	block := &block.Block{
//		Height:       height,
//		PrevHash:     prevHash,
//		Transactions: txs,
//		Root:         root,
//		Version:      1,
//		Timestamp:    time.Now().Unix(),
//		Miner:        minaddr,
//	}
//	block.SetHash()
//	logger.Info("end to new block")
//	return block, nil
//}
//
//// AddBlock 向数据库添加新的block数据，minaddr矿工地址
//func (bc *Blockchain) AddBlock(block *block.Block, minaddr []byte) error {
//	logger.Info("addBlock", zap.Uint64("blockHeight", block.Height))
//	bc.mu.Lock()
//	defer bc.mu.Unlock()
//
//	DBTransaction := bc.db.NewTransaction()
//	defer DBTransaction.Cancel()
//	var err error
//	var height, prevHeight uint64
//	//拿出块高
//	prevHeight, err = bc.getHeight()
//	if err != nil {
//		logger.Error("failed to get height", zap.Error(err))
//		return err
//	}
//
//	height = prevHeight + 1
//	if block.Height != height {
//		return fmt.Errorf("height error:current height=%d,commit height=%d", prevHeight, block.Height)
//	}
//
//	//高度->哈希
//	hash := block.Hash
//	if err = DBTransaction.Set(append(HeightPrefix, miscellaneous.E64func(height)...), hash); err != nil {
//		logger.Error("Failed to set height and hash", zap.Error(err))
//		return err
//	}
//
//	//重置块高
//	DBTransaction.Del(HeightKey)
//	DBTransaction.Set(HeightKey, miscellaneous.E64func(height))
//
//	// 获取pck和dkto的总数
//	pckTotal, err := getPckTotal(DBTransaction)
//	if err != nil {
//		logger.Error("failed to get total of pck")
//		return err
//	}
//
//	dKtoTotal, err := getDKtoTotal(DBTransaction)
//	if err != nil {
//		logger.Error("failed to get total of dkto")
//		return err
//	}
//
//	CDBTransaction := bc.cdb.NewTransaction()
//	defer CDBTransaction.Cancel()
//
//	bc.orderEngine.SetTransaction(DBTransaction, CDBTransaction)
//	for index, tx := range block.Transactions {
//		if eo := tx.ExchangeOrder; eo != nil {
//			logger.Info("into add order")
//
//			ord := order.Order{
//				Sell:       eo.Sell,
//				Buy:        eo.Buy,
//				SellName:   eo.SellName,
//				BuyName:    eo.BuyName,
//				SellNumber: eo.SellNumber,
//				BuyNumber:  eo.BuyNumber,
//				SellFee:    eo.SellFee,
//				BuyFee:     eo.BuyFee,
//				Expiration: eo.Expiration,
//				Sign:       eo.Sign,
//			}
//			receipt := make([]byte, 256, 256)
//			orderHash, err := bc.orderEngine.Create(ord, receipt)
//			if err != nil {
//				logger.Error("failed create order", zap.Error(err))
//			}
//			logger.Info("eo info", zap.String("sell", hex.EncodeToString(ord.Sell)), zap.String("buy", hex.EncodeToString(ord.Buy)),
//				zap.String("sellName", string(ord.SellName)), zap.String("buyName", string(ord.BuyName)), zap.Uint64("sellNumber", ord.SellNumber),
//				zap.Uint64("buyNumber", ord.BuyNumber), zap.Uint64("sellFee", ord.SellFee), zap.Uint64("buyFee", ord.BuyFee), zap.Int64("Expiration", ord.Expiration),
//				zap.String("sign", hex.EncodeToString(ord.Sign)))
//			tx.ExchangeOrder.Hash = orderHash
//			logger.Info("eo hash", zap.String("hash", hex.EncodeToString(orderHash)))
//		} else if to := tx.TakerOrder; to != nil {
//			proof := make([]byte, 64, 64)
//			if up, err := bc.orderEngine.Exchange(to.OrderHash, to.Taker, to.Sign, to.BuyFee, to.SellFee, to.BuyNumber, to.SellNumber, proof); err != nil {
//				logger.Error("failed to exchange order", zap.Error(err), zap.String("orderHash", hex.EncodeToString(to.OrderHash)),
//					zap.String("taker", types.PublicKeyToAddress(to.Taker)), zap.String("sign", hex.EncodeToString(to.Sign)),
//					zap.Uint64("buyfee", to.BuyFee), zap.Uint64("sellfee", to.SellFee), zap.Uint64("buyNum", to.BuyNumber), zap.Uint64("sellNum", to.SellNumber))
//			} else {
//				tx.TakerOrder.Traded = true
//				tx.TakerOrder.UnsoldQuantity = up
//			}
//		}
//
//		if tx.IsCoinBaseTransaction() {
//			if err = setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			if err := setToAccount(DBTransaction, tx); err != nil {
//				logger.Error("Failed to set account", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.Uint64("amount", tx.Amount))
//				return err
//			}
//		} else if tx.IsFreezeTransaction() || tx.IsUnfreezeTransaction() {
//			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			if err := setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			nonce := tx.Nonce + 1
//			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			var frozenBalBytes []byte
//			frozenBal, _ := getFreezeBalance(DBTransaction, tx.To.Bytes())
//			if tx.IsFreezeTransaction() {
//				frozenBalBytes = miscellaneous.E64func(tx.Amount + frozenBal)
//				/* 			//投票记录处理
//				vote := NewVote(tx)
//				SetVote(*vote, DBTransaction) */
//			} else {
//				frozenBalBytes = miscellaneous.E64func(frozenBal - tx.Amount)
//			}
//			if err := setFreezeBalance(DBTransaction, tx.To.Bytes(), frozenBalBytes); err != nil {
//				logger.Error("Faile to freeze balance", zap.String("address", tx.To.String()),
//					zap.Uint64("amount", tx.Amount))
//				return err
//			}
//		} else if tx.IsConvertPckTransaction() || tx.IsConvertKtoTransaction() {
//			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			//更新nonce,block中txs必须是有序的
//			nonce := tx.Nonce + 1
//			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			if tx.IsConvertPckTransaction() {
//				if err := setConvertPck(DBTransaction, tx.From.Bytes(), tx.KtoNum, tx.PckNum); err != nil {
//					logger.Error("Failed to set pck", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.Uint64("pck", tx.PckNum), zap.Uint64("dkto", tx.KtoNum))
//					return err
//				}
//
//				if util.Uint64AddOverflow(pckTotal, tx.PckNum) && util.Uint64AddOverflow(dKtoTotal, tx.KtoNum) {
//					logger.Error("faile to verify total")
//					return errors.New("uint64 overflow")
//				}
//				pckTotal += tx.PckNum
//				dKtoTotal += tx.KtoNum
//			} else {
//				if err := setConvertKto(DBTransaction, tx.From.Bytes(), tx.KtoNum, tx.PckNum); err != nil {
//					logger.Error("Failed to set dkto", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.Uint64("pck", tx.PckNum), zap.Uint64("dkto", tx.KtoNum))
//					return err
//				}
//				if pckTotal < tx.PckNum && dKtoTotal < tx.KtoNum {
//					logger.Error("faile to verify total")
//					return errors.New("uint64 overflow")
//				}
//				pckTotal -= tx.PckNum
//				dKtoTotal -= tx.KtoNum
//			}
//		} else if tx.IsContractTrade() { /*********Contract trade********/
//			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			if err := setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			nonce := tx.Nonce + 1
//			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			//set block info into evm
//			bc.evm.SetBlockInfo(block.Height, block.Miner.String(), uint64(block.Timestamp)) //,tx.GasLimit)
//
//			/*leftGas,*/
//			err := bc.HandleContract(block, DBTransaction, tx, index)
//			if err != nil {
//				logger.Error("HandleContract error:", zap.Error(err))
//				break
//			}
//
//			//minner fee
//			if err = setMinerFee(DBTransaction, minaddr, tx.Fee); err != nil { // fee = tx.GasLimit-leftFas
//				logger.Error("Failed to set fee", zap.Error(err), zap.String("from address", tx.From.String()), zap.Uint64("fee", tx.Fee))
//				return err
//			}
//			//更新余额
//			if err := setAccount(DBTransaction, tx); err != nil {
//				logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//		} else if tx.IsBindingAddress() { //binding eth&kto address
//			fmt.Println("bindingAddress==========", tx.From.String(), tx.To.String(), tx.EthFrom)
//			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			if err := setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			nonce := tx.Nonce + 1
//			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//			fmt.Println("bindingAddress==========", tx.EthFrom, tx.From.String(), tx.EthFrom.Bytes())
//			err := bindingAddress(DBTransaction, tx.EthFrom.Bytes(), tx.From.Bytes())
//			if err != nil {
//				logger.Error("Failed to bindingAddress", zap.Error(err), zap.String("from address", tx.From.String()), zap.String("eth address", tx.EthFrom.Hex()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				break
//			}
//
//			//更新余额
//			if err := setAccount(DBTransaction, tx); err != nil {
//				logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//		} else if tx.IsEthSignedTransaction() {
//			fmt.Println("EthSignedTransaction==========", tx.From.String(), tx.To.String(), tx.EthFrom)
//			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			if err := setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			nonce := tx.Nonce + 1
//			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			//更新余额
//			if err := setAccount(DBTransaction, tx); err != nil {
//				logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//
//			}
//		} else {
//			if tx.IsTokenTransaction() {
//				sc := parser.Parser([]byte(tx.Script))
//				e, err := exec.New(CDBTransaction, sc, tx.From.String())
//				if err != nil {
//					logger.Error("Failed to new exec", zap.String("script", tx.Script),
//						zap.String("from address", tx.From.String()))
//					tx.Script += " InsufficientBalance"
//					block.Transactions[index] = tx
//				}
//
//				if e != nil {
//					if err = e.Flush(CDBTransaction); err != nil {
//						logger.Error("Failed to flush exec", zap.String("script", tx.Script),
//							zap.String("from address", tx.From.String()))
//						tx.Script += " ExecutionFailed"
//						block.Transactions[index] = tx
//					}
//				}
//
//				if err = setMinerFee(DBTransaction, minaddr, tx.Fee); err != nil {
//					logger.Error("Failed to set fee", zap.Error(err), zap.String("script", tx.Script),
//						zap.String("from address", tx.From.String()), zap.Uint64("fee", tx.Fee))
//					return err
//				}
//			}
//
//			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			if err := setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			//更新nonce,block中txs必须是有序的
//			nonce := tx.Nonce + 1
//			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			//更新余额
//			if err := setAccount(DBTransaction, tx); err != nil {
//				logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//		}
//
//		// if err := setTxList(DBTransaction, tx); err != nil {
//		// 	logger.Error("Failed to set block data", zap.String("from", tx.From.String()), zap.Uint64("nonce", tx.Nonce))
//		// 	return err
//		// }
//	}
//
//	//哈希-> 块
//	if err = DBTransaction.Set(hash, block.Serialize()); err != nil {
//		logger.Error("Failed to set block", zap.Error(err))
//		return err
//	}
//
//	/* 	//固定周期处理投票结果
//	   	if height%(30*24*60*60) == 0 {
//	   		//处理投票结果
//	   		Voteresult(DBTransaction)
//		   } */
//
//	if err := setPckAndDktoToatal(DBTransaction, pckTotal, dKtoTotal); err != nil {
//		logger.Error("failed to set total", zap.Error(err))
//		return err
//	}
//
//	{
//		//TODO:commit
//		logger.Info("end addBlock", zap.Uint64("blockHeight", block.Height))
//		if err := DBTransaction.Commit(); err != nil {
//			logger.Error("commit ktodb", zap.Error(err), zap.Uint64("block number", block.Height))
//			return err
//		}
//
//		if err := bc.orderEngine.Commit(); err != nil {
//			logger.Error("conmmit engine", zap.Error(err), zap.Uint64("block number", block.Height))
//			return err
//		}
//	}
//
//	return CDBTransaction.Commit()
//}
//
//// GetNonce 获取address的nonce
//func (bc *Blockchain) GetNonce(address []byte) (uint64, error) {
//	bc.mu.RLock()
//
//	nonceBytes, err := bc.db.Mget(NonceKey, address)
//	if err == store.NotExist {
//		bc.mu.RUnlock()
//		return 1, bc.setNonce(address, 1)
//	} else if err != nil {
//		return 0, err
//	}
//	bc.mu.RUnlock()
//
//	return miscellaneous.D64func(nonceBytes)
//}
//
//// func (bc *Blockchain) getNonce(address []byte) (uint64, error) {
//// 	nonceBytes, err := bc.db.Mget(NonceKey, address)
//// 	if err == store.NotExist {
//// 		return 1, bc.setNonce(address, 1)
//// 	} else if err != nil {
//// 		return 0, err
//// 	}
//
//// 	return miscellaneous.D64func(nonceBytes)
//// }
//
//func (bc *Blockchain) setNonce(address []byte, nonce uint64) error {
//	bc.mu.Lock()
//	defer bc.mu.Unlock()
//
//	nonceBytes := miscellaneous.E64func(nonce)
//	return bc.db.Mset(NonceKey, address, nonceBytes)
//}
//
//// GetBalance 获取address的余额
///*
//func (bc *Blockchain) GetBalance(address []byte) (uint64, error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//
//	return bc.getBalance(address)
//}
//
//func (bc *Blockchain) getBalance(address []byte) (uint64, error) {
//	balanceBytes, err := bc.db.Get(address)
//	if err == store.NotExist {
//		return 0, nil
//	} else if err != nil {
//		return 0, err
//	}
//
//	return miscellaneous.D64func(balanceBytes)
//}*/
//
//// GetHeight 获取当前块高
//func (bc *Blockchain) GetHeight() (height uint64, err error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//	return bc.getHeight()
//}
//
//func (bc *Blockchain) getHeight() (uint64, error) {
//	heightBytes, err := bc.db.Get(HeightKey)
//	if err == store.NotExist {
//		return 0, nil
//	} else if err != nil {
//		return 0, err
//	}
//	return miscellaneous.D64func(heightBytes)
//}
//
//// GetHash 获取块高对应的hash
//func (bc *Blockchain) GetHash(height uint64) (hash []byte, err error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//
//	return bc.db.Get(append(HeightPrefix, miscellaneous.E64func(height)...))
//}
//
//// GetBlockByHash 获取hash对应的块数据
//func (bc *Blockchain) GetBlockByHash(hash []byte) (*block.Block, error) { //
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//
//	blockData, err := bc.db.Get(hash)
//	if err != nil {
//		return nil, err
//	}
//	return block.Deserialize(blockData)
//}
//
//// GetBlockByHeight 获取块高对应的块
//func (bc *Blockchain) GetBlockByHeight(height uint64) (*block.Block, error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//	return bc.getBlockByheight(height)
//}
//
//func (bc *Blockchain) getBlockByheight(height uint64) (*block.Block, error) {
//	if height < 1 {
//		return nil, errors.New("parameter error")
//	}
//
//	// 1、先获取到hash
//	hash, err := bc.db.Get(append(HeightPrefix, miscellaneous.E64func(height)...))
//	if err != nil {
//		return nil, err
//	}
//
//	// 2、通过hash获取block
//	blockData, err := bc.db.Get(hash)
//	if err != nil {
//		return nil, err
//	}
//
//	return block.Deserialize(blockData)
//}
//
//func (bc *Blockchain) GBbyHeight(height uint64) (*block.Block, error) {
//	if height < 1 {
//		return nil, errors.New("parameter error")
//	}
//
//	// 1、先获取到hash
//	hash, err := bc.db.Get(append(HeightPrefix, miscellaneous.E64func(height)...))
//	if err != nil {
//		return nil, err
//	}
//
//	// 2、通过hash获取block
//	blockData, err := bc.db.Get(hash)
//	if err != nil {
//		return nil, err
//	}
//
//	return block.Deserialize(blockData)
//}
//
///*
//// GetTransactions 获取从start到end的所有交易
//func (bc *Blockchain) GetTransactions(start, end int64) ([]*transaction.Transaction, error) {
//	//获取hash的交易
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//
//	hashList, err := bc.db.Lrange(TxListName, start, end)
//	if err != nil {
//		logger.Error("failed to get txlist", zap.Error(err))
//		return nil, err
//	}
//
//	transactions := make([]*transaction.Transaction, 0, len(hashList))
//	for _, hash := range hashList {
//		txBytes, err := bc.db.Get(hash)
//		if err != nil {
//			return nil, err
//		}
//
//		transaction := &transaction.Transaction{}
//		if err := json.Unmarshal(txBytes, transaction); err != nil {
//			logger.Error("Failed to unmarshal bytes", zap.Error(err))
//			return nil, err
//		}
//
//		transactions = append(transactions, transaction)
//	}
//
//	return transactions, err
//}
//*/
//// GetTransactions 获取从start到end的所有交易
//func (bc *Blockchain) GetTransactions(start, end int64) ([]*transaction.Transaction, error) {
//	//获取hash的交易
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//
//	hashList, err := bc.db.Lrange(TxListName, start, end)
//	if err != nil {
//		logger.Error("failed to get txlist", zap.Error(err))
//		return nil, err
//	}
//
//	transactions := make([]*transaction.Transaction, 0, len(hashList))
//	for _, hash := range hashList {
//		txBytes, err := bc.getTransactionByHash(hash)
//		if err != nil {
//			return nil, err
//		}
//
//		// transaction := &transaction.Transaction{}
//		// if err := json.Unmarshal(txBytes, transaction); err != nil {
//		// 	logger.Error("Failed to unmarshal bytes", zap.Error(err))
//		// 	return nil, err
//		// }
//
//		transactions = append(transactions, txBytes)
//	}
//
//	return transactions, err
//}
//
//// GetTransactionByHash 获取交易哈希对应的交易
//func (bc *Blockchain) GetTransactionByHash(hash []byte) (*transaction.Transaction, error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//	return bc.getTransactionByHash(hash)
//}
//
//func (bc *Blockchain) getTransactionByHash(hash []byte) (*transaction.Transaction, error) {
//
//	Hi, err := bc.db.Get(hash)
//	if err != nil {
//		logger.Error("failed to get hash", zap.Error(err))
//		return nil, err
//	}
//	var txindex TXindex
//	err = json.Unmarshal(Hi, &txindex)
//	if err != nil {
//		logger.Error("Failed to unmarshal bytes", zap.Error(err))
//		return nil, err
//	}
//	// bh, err := miscellaneous.D64func(Hi.Height)
//	// if err != nil {
//	// 	logger.Error("failed to get hash", zap.Error(err))
//	// 	return nil, err
//	// }
//	b, err := bc.getBlockByheight(txindex.Height)
//	if err != nil {
//		logger.Error("failed to getblock height", zap.Error(err), zap.Uint64("height", txindex.Height))
//		return nil, err
//	}
//
//	//transaction := &transaction.Transaction{}
//	tx := b.Transactions[txindex.Index]
//	// err = json.Unmarshal(tx, transaction)
//	// if err != nil {
//	// 	return nil, err
//	// }
//
//	return tx, nil
//}
//
//// // GetTransactionByAddr 获取address从start到end的所有交易
//// func (bc *Blockchain) GetTransactionByAddr(address []byte, start, end int64) ([]*transaction.Transaction, error) {
//// 	bc.mu.RLock()
//// 	defer bc.mu.RUnlock()
//
//// 	txHashList, err := bc.db.Lrange(append(AddrListPrefix, address...), start, end)
//// 	if err != nil {
//// 		logger.Error("failed to get addrhashlist", zap.Error(err))
//// 		return nil, err
//// 	}
//
//// 	transactions := make([]*transaction.Transaction, 0, len(txHashList))
//// 	for _, hash := range txHashList {
//// 		txBytes, err := bc.db.Get(hash)
//// 		if err != nil {
//// 			logger.Error("Failed to get transaction", zap.Error(err), zap.ByteString("hash", hash))
//// 			return nil, err
//// 		}
//// 		var tx transaction.Transaction
//// 		if err := json.Unmarshal(txBytes, &tx); err != nil {
//// 			logger.Error("Failed to unmarshal bytes", zap.Error(err))
//// 			return nil, err
//// 		}
//// 		transactions = append(transactions, &tx)
//// 	}
//
//// 	return transactions, nil
//// }
//
//// GetTransactionByAddr 获取address从start到end的所有交易
//func (bc *Blockchain) GetTransactionByAddr_rm(address []byte, start, end int64) ([]*transaction.Transaction, error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//	//Mkeys([]byte) ([][]byte, error)
//	txHashList, err := bc.db.Mkeys(address)
//	if err != nil {
//		logger.Error("failed to get addrhashlist", zap.Error(err))
//		return nil, err
//	}
//
//	transactions := make([]*transaction.Transaction, 0, len(txHashList))
//	ltx := len(txHashList)
//	if uint64(end) > uint64(ltx) {
//		end = int64(ltx)
//	}
//
//	for i := start; i < end; i++ {
//		txBytes, err := bc.getTransactionByHash(txHashList[i])
//		if err != nil {
//			logger.Error("Failed to get transaction", zap.Error(err), zap.ByteString("hash", txHashList[i]))
//			return nil, err
//		}
//
//		transactions = append(transactions, txBytes)
//	}
//
//	return transactions, nil
//}
//
//func (bc *Blockchain) GetTransactionByAddr(address []byte, start, end int64) ([]*transaction.Transaction, error) {
//	return nil, errors.New("interface is obsolete")
//}
//
//// GetContractDB 获取忽而学数据库对象
//func (bc *Blockchain) GetContractDB() store.DB {
//	return bc.cdb
//}
//
//// GetMaxBlockHeight 获取块高
//func (bc *Blockchain) GetMaxBlockHeight() (uint64, error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//
//	heightBytes, err := bc.db.Get(HeightKey)
//	if err == store.NotExist {
//		return 0, nil
//	} else if err != nil {
//		return 0, err
//	}
//
//	return miscellaneous.D64func(heightBytes)
//}
//
//// GetTokenBalance 获取代币余额
//func (bc *Blockchain) GetTokenBalance(address, symbol []byte) (uint64, error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//
//	bal, err := exec.Balance(bc.cdb, string(symbol), string(address))
//	if err != nil && err != store.NotExist {
//		return 0, err
//	}
//	return bal, nil
//}
//
//// GetFrozenTokenBal 获取已冻结代币的金额
//func (bc *Blockchain) GetFrozenTokenBal(address, symbol []byte) (uint64, error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//
//	bal, err := exec.Freeze(bc.cdb, string(symbol), string(address))
//	if err != nil && err != store.NotExist {
//		return 0, err
//	}
//	return bal, nil
//}
//
//// GetAvailableTokenBal 获取代币可用余额
//func (bc *Blockchain) GetAvailableTokenBal(address, symbol []byte) (uint64, error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//	var err error
//	var bal, frozenBal uint64
//	bal, err = exec.Balance(bc.cdb, string(symbol), string(address))
//	if err != nil && err != store.NotExist {
//		return 0, err
//	}
//
//	frozenBal, err = exec.Freeze(bc.cdb, string(symbol), string(address))
//	if err != nil && err != store.NotExist {
//		return 0, err
//	}
//
//	if bal < frozenBal {
//		return 0, nil
//	}
//
//	return bal - frozenBal, nil
//}
//
//func setMinerFee(tx store.Transaction, to []byte, amount uint64) error {
//	tobalance, err := tx.Get(to)
//	if err == store.NotExist {
//		tobalance = miscellaneous.E64func(0)
//	} else if err != nil {
//		return err
//	}
//
//	toBalance, _ := miscellaneous.D64func(tobalance)
//	if MAXUINT64-toBalance < amount {
//		return fmt.Errorf("amount is too large,max int64(%d)-balance(%d) < amount(%d)", MAXUINT64, toBalance, amount)
//	}
//	toBalanceBytes := miscellaneous.E64func(toBalance + amount)
//
//	return setBalance(tx, to, toBalanceBytes)
//}
//
//// Distr 出币分配
//func Distr(txs []*transaction.Transaction, minaddr, Ds, Cm, QTJ types.Address, height uint64) []*transaction.Transaction {
//	//TODO:避免魔数存在
//	var orderIndexList []int
//	var total uint64 = 49460000000
//	x := height / 31536000 //矿工奖励衰减周期
//
//	for i := 0; uint64(i) < x; i++ {
//		total = total * 8 / 10
//	}
//	each, mod := total/10, total%10
//
//	for i, tx := range txs {
//		if tx.IsOrderTransaction() && tx.Order.Vertify(QTJ) {
//			orderIndexList = append(orderIndexList, i)
//		}
//	}
//
//	if len(orderIndexList) != 0 {
//		fAmonut, fMod := each/uint64(len(orderIndexList)), each%uint64(len(orderIndexList)) //10% 订单用户
//		for _, orderIndex := range orderIndexList {
//			txs = append(txs, transaction.NewCoinBaseTransaction(txs[orderIndex].Order.Address, fAmonut))
//		}
//
//		dsAmount := each + fMod //10% 电商
//		txs = append(txs, transaction.NewCoinBaseTransaction(Ds, dsAmount))
//	} else {
//		dsAmount := each * 2 //20% 电商
//		txs = append(txs, transaction.NewCoinBaseTransaction(Ds, dsAmount))
//	}
//
//	jsAmount := each*4 + mod //40% 技术
//	txs = append(txs, transaction.NewCoinBaseTransaction(minaddr, jsAmount))
//
//	sqAmount := each * 4 //40% 社区
//	//	SetIncome(Samount)
//	txs = append(txs, transaction.NewCoinBaseTransaction(Cm, sqAmount))
//
//	return txs
//}
//
//func setTxbyaddr(DBTransaction store.Transaction, addr []byte, tx transaction.Transaction) error {
//	// txBytes, _ := json.Marshal(tx)
//	// return DBTransaction.Mset(addr, tx.Hash, txBytes)
//	listNmae := append(AddrListPrefix, addr...)
//	_, err := DBTransaction.Llpush(listNmae, tx.Hash)
//	return err
//}
//
//func setTxbyaddrKV(DBTransaction store.Transaction, addr []byte, tx transaction.Transaction, index uint64) error {
//	// txBytes, _ := json.Marshal(tx)
//	// return DBTransaction.Mset(addr, tx.Hash, txBytes)
//	DBTransaction.Mset(addr, tx.Hash, []byte(""))
//	txindex := &TXindex{
//		Height: tx.BlockNumber,
//		Index:  index,
//	}
//	tdex, err := json.Marshal(txindex)
//	if err != nil {
//		logger.Error("Failed Marshal txindex", zap.Error(err))
//		return err
//	}
//	DBTransaction.Set(tx.Hash, tdex)
//	return err
//}
//
///*
//func setNonce(DBTransaction store.Transaction, addr, nonce []byte) error {
//	DBTransaction.Mdel(NonceKey, addr)
//	return DBTransaction.Mset(NonceKey, addr, nonce)
//}
//*/
//
//func bindingAddress(DBTransaction store.Transaction, ethaddr, ktoaddr []byte) error {
//	DBTransaction.Mdel(BindingKey, ethaddr)
//	return DBTransaction.Mset(BindingKey, ethaddr, ktoaddr)
//}
//func setTxList(DBTransaction store.Transaction, tx *transaction.Transaction) error {
//	//TxList->txhash
//	if _, err := DBTransaction.Llpush(TxListName, tx.Hash); err != nil {
//		logger.Error("Failed to push txhash", zap.Error(err))
//		return err
//	}
//
//	//交易hash->交易数据
//	txBytes, _ := json.Marshal(tx)
//	if err := DBTransaction.Set(tx.Hash, txBytes); err != nil {
//		logger.Error("Failed to set transaction", zap.Error(err))
//		return err
//	}
//
//	return nil
//}
//
///*
//func setToAccount(dbTransaction store.Transaction, transaction *transaction.Transaction) error {
//	var balance uint64
//	balanceBytes, err := dbTransaction.Get(transaction.To.Bytes())
//	if err != nil {
//		balance = 0
//	} else {
//		balance, err = miscellaneous.D64func(balanceBytes)
//		if err != nil {
//			return err
//		}
//	}
//
//	if MAXUINT64-transaction.Amount < balance {
//		return fmt.Errorf("not sufficient funds")
//	}
//	newBalanceBytes := miscellaneous.E64func(balance + transaction.Amount)
//	if err := setBalance(dbTransaction, transaction.To.Bytes(), newBalanceBytes); err != nil {
//		return err
//	}
//
//	return nil
//}
//*/
///*
//func setAccount(DBTransaction store.Transaction, tx *transaction.Transaction) error {
//	from, to := tx.From.Bytes(), tx.To.Bytes()
//
//	fromBalBytes, _ := DBTransaction.Get(from)
//	fromBalance, _ := miscellaneous.D64func(fromBalBytes)
//	if tx.IsTokenTransaction() {
//		if fromBalance < tx.Amount+tx.Fee {
//			return fmt.Errorf("not sufficient funds,hash:%s,from balance(%d) < amount(%d)+fee(%d)",
//				hex.EncodeToString(tx.Hash), fromBalance, tx.Amount, tx.Fee)
//		}
//		fromBalance -= tx.Amount + tx.Fee
//	} else {
//		if fromBalance < tx.Amount {
//			return fmt.Errorf("not sufficient funds,hash:%s,from balance(%d) < amount(%d)",
//				hex.EncodeToString(tx.Hash), fromBalance, tx.Amount)
//		}
//		fromBalance -= tx.Amount
//	}
//
//	tobalance, err := DBTransaction.Get(to)
//	if err != nil {
//		setBalance(DBTransaction, to, miscellaneous.E64func(0))
//		tobalance = miscellaneous.E64func(0)
//	}
//
//	toBalance, _ := miscellaneous.D64func(tobalance)
//
//	if MAXUINT64-toBalance < tx.Amount {
//		return fmt.Errorf("amount is too large,hash:%s,max int64(%d)-balance(%d) < amount(%d)", tx.Hash, MAXUINT64, toBalance, tx.Amount)
//	}
//	toBalance += tx.Amount
//
//	Frombytes := miscellaneous.E64func(fromBalance)
//	Tobytes := miscellaneous.E64func(toBalance)
//
//	if err := setBalance(DBTransaction, from, Frombytes); err != nil {
//		return err
//	}
//	if err := setBalance(DBTransaction, to, Tobytes); err != nil {
//		return err
//	}
//
//	return nil
//}
//*/
///*
//func setBalance(tx store.Transaction, addr, balance []byte) error {
//	tx.Del(addr)
//	return tx.Set(addr, balance)
//}
//*/
//
///*
//func setBalance(s store.StateTransaction, addr, balance []byte) error {
//	return s.Set(addr, balance)
//}
//*/
///*
//func setFreezeBalance(tx store.Transaction, addr, freezeBal []byte) error {
//	return tx.Mset(FreezeKey, addr, freezeBal)
//}
//*/
//
////GetFreezeBalance 获取被冻结的金额
///*
//func (bc *Blockchain) GetFreezeBalance(address []byte) (uint64, error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//	return bc.getFreezeBalance(address)
//}
//
//func (bc *Blockchain) getFreezeBalance(address []byte) (uint64, error) {
//	freezeBalBytes, err := bc.db.Mget(FreezeKey, address)
//	if err == store.NotExist {
//		return 0, nil
//	} else if err != nil {
//		return 0, err
//	}
//	return miscellaneous.D64func(freezeBalBytes)
//}
//*/
//func getFreezeBalance(tx store.Transaction, address []byte) (uint64, error) {
//	freezeBalBytes, err := tx.Mget(FreezeKey, address)
//	if err == store.NotExist {
//		return 0, nil
//	} else if err != nil {
//		return 0, err
//	}
//	return miscellaneous.D64func(freezeBalBytes)
//}
//
//// CalculationResults 计算出该block上链后个地址的可用余额，如果余额不正确则返回错误
//func (bc *Blockchain) CalculationResults(block *block.Block) ([]byte, error) {
//	//TODO:计算出余额后，进行hash
//	var ok bool
//	var err error
//	var avlBalance, frozenBalance uint64
//	var pckDkto struct {
//		pck  uint64
//		dkto uint64
//	}
//	dexAddrStr := types.PublicKeyToAddress(bc.orderEngine.Addr)
//	DexFeeAddr, err := types.StringToAddress(dexAddrStr)
//	if err != nil {
//		logger.Error("failed parse dex fee address", zap.Error(err), zap.String("dex fee addr", string(bc.orderEngine.Addr)))
//		return nil, errors.New("failed parse dex fee address")
//	}
//
//	//block.Results = make(map[string]uint64)
//	avlBalanceResults := make(map[string]uint64)
//	frozenBalanceResults := make(map[string]uint64)
//	pckDktoResults := make(map[string]struct {
//		pck  uint64
//		dkto uint64
//	})
//	for _, tx := range block.Transactions {
//		//1、from余额计算
//		if tx.IsTransferTrasnaction() || tx.IsConvertKtoTransaction() || tx.IsConvertPckTransaction() || tx.IsContractTrade() || tx.IsBindingAddress() || tx.IsEthSignedTransaction() {
//
//			if avlBalance, ok = avlBalanceResults[tx.From.String()]; !ok {
//				balance, err := bc.GetBalance(tx.From.Bytes())
//				if err != nil {
//					return nil, err
//				}
//
//				frozenBalance, err := bc.GetFreezeBalance(tx.From.Bytes())
//				if err != nil {
//					return nil, err
//				}
//
//				if util.Uint64SubOverflow(balance, frozenBalance) {
//					logger.Info("sub overflow", zap.Uint64("balance", balance), zap.Uint64("frozen balance", frozenBalance))
//					return nil, errors.New("insufficient balance")
//				}
//
//				avlBalance = balance - frozenBalance
//			}
//
//			if tx.IsConvertKtoTransaction() || tx.IsConvertPckTransaction() {
//				if pckDkto, ok = pckDktoResults[tx.From.String()]; !ok {
//					pckDkto.dkto, err = bc.GetDKto(tx.From.Bytes())
//					if err != nil {
//						return nil, err
//					}
//
//					pckDkto.pck, err = bc.GetPck(tx.From.Bytes())
//					if err != nil {
//						return nil, err
//					}
//				}
//			}
//
//			if tx.IsTransferTrasnaction() || tx.IsContractTrade() || tx.IsBindingAddress() {
//				if util.Uint64SubOverflow(avlBalance, tx.Amount, tx.Fee) {
//					logger.Info("sub overflow", zap.Uint64("avaliable balance", avlBalance), zap.Uint64("amount", tx.Amount),
//						zap.Uint64("fee", tx.Fee))
//					return nil, errors.New("insufficient balance")
//				}
//				avlBalanceResults[tx.From.String()] = avlBalance - tx.Amount - tx.Fee
//			} else if tx.IsConvertPckTransaction() {
//				if avlBalance < tx.KtoNum || util.Uint64AddOverflow(pckDkto.pck, tx.PckNum) ||
//					util.Uint64AddOverflow(pckDkto.dkto, tx.KtoNum) {
//					logger.Info("sub overflow", zap.Uint64("avaliable balance", avlBalance), zap.Uint64("ktonum", tx.KtoNum),
//						zap.Uint64("pck balance", pckDkto.pck), zap.Uint64("pck number", tx.PckNum), zap.Uint64("dkto balance", pckDkto.dkto))
//					return nil, errors.New("insufficient balance")
//				}
//				avlBalanceResults[tx.From.String()] = avlBalance - tx.KtoNum
//				pckDktoResults[tx.From.String()] = struct {
//					pck  uint64
//					dkto uint64
//				}{pckDkto.pck + tx.PckNum, pckDkto.dkto + tx.KtoNum}
//			} else if tx.IsConvertKtoTransaction() {
//				if pckDkto.pck < tx.PckNum || pckDkto.dkto < tx.KtoNum || util.Uint64AddOverflow(avlBalance, tx.KtoNum) {
//					logger.Info("add overflow", zap.Uint64("avaliable balance", avlBalance), zap.Uint64("ktonum", tx.KtoNum),
//						zap.Uint64("pck balance", pckDkto.pck), zap.Uint64("pck number", tx.PckNum), zap.Uint64("dkto balance", pckDkto.dkto))
//					return nil, errors.New("insufficient balance")
//				}
//				avlBalanceResults[tx.From.String()] = avlBalance + tx.KtoNum
//				pckDktoResults[tx.From.String()] = struct {
//					pck  uint64
//					dkto uint64
//				}{pckDkto.pck - tx.PckNum, pckDkto.dkto - tx.KtoNum}
//			}
//		}
//
//		//2、to余额计算
//		if !tx.IsConvertKtoTransaction() && !tx.IsConvertPckTransaction() {
//			if avlBalance, ok = avlBalanceResults[tx.To.String()]; !ok {
//				balance, err := bc.GetBalance(tx.To.Bytes())
//				if err != nil {
//					return nil, err
//				}
//
//				frozenBalance, err := bc.GetFreezeBalance(tx.To.Bytes())
//				if err != nil {
//					return nil, err
//				}
//
//				if util.Uint64SubOverflow(balance, frozenBalance) {
//					logger.Info("sub overflow", zap.String("address", tx.To.String()),
//						zap.Uint64("balance", balance), zap.Uint64("frozen balance", frozenBalance))
//					return nil, errors.New("insufficient balance")
//				}
//				//logger.Info("Balance information", zap.String("address", tx.To.String()),
//				//	zap.Uint64("balance", balance), zap.Uint64("frozen balance", frozenBalance))
//				avlBalance = balance - frozenBalance
//			}
//
//			if frozenBalance, ok = frozenBalanceResults[tx.To.String()]; !ok {
//				frozenBalance, err = bc.getFreezeBalance(tx.To.Bytes())
//				if err != nil {
//					return nil, err
//				}
//			}
//
//			if tx.IsCoinBaseTransaction() || tx.IsTransferTrasnaction() {
//				avlBalanceResults[tx.To.String()] = avlBalance + tx.Amount
//			} else if tx.IsFreezeTransaction() {
//				//TODO:处理冻结金额大于余额的情况
//				if avlBalance < tx.Amount {
//					logger.Info("sub overflow", zap.Uint64("avaliable balance", avlBalance), zap.Uint64("amount", tx.Amount))
//					return nil, errors.New("insufficient balance")
//				}
//				avlBalanceResults[tx.To.String()] = avlBalance - tx.Amount
//				frozenBalanceResults[tx.To.String()] = frozenBalance + tx.Amount
//			} else if tx.IsUnfreezeTransaction() {
//				if frozenBalance < tx.Amount {
//					logger.Info("sub overflow", zap.Uint64("frozen balance", frozenBalance), zap.Uint64("amount", tx.Amount))
//					return nil, errors.New("insufficient frozen balance")
//				}
//				frozenBalanceResults[tx.To.String()] = frozenBalance - tx.Amount
//				avlBalanceResults[tx.To.String()] = avlBalance + tx.Amount
//			} else if tx.IsContractTrade() || tx.IsBindingAddress() || tx.IsEthSignedTransaction() {
//				avlBalanceResults[tx.To.String()] = avlBalance + tx.Amount
//			} else {
//				return nil, errors.New("wrong transaction type")
//			}
//		}
//
//		//3.calculate order
//		//var orderTx store.Transaction
//		if tx.TakerOrder != nil {
//			// if orderTx == nil {
//			// 	orderTx = bc.orderEngine.GetTransaction()
//			// 	defer orderTx.Cancel()
//			// }
//			to := tx.TakerOrder
//			// eo, err := bc.orderEngine.GetOrder(to.OrderHash, orderTx)
//			eo, err := bc.orderEngine.GetOrder(to.OrderHash)
//			if err != nil {
//				logger.Error("get order err", zap.Error(err), zap.String("order hash", hex.EncodeToString(to.OrderHash)))
//				continue
//			}
//
//			sellAddr, _ := types.StringToAddress(types.PublicKeyToAddress(eo.Sell))
//			buyAddr, _ := types.StringToAddress(types.PublicKeyToAddress(to.Taker))
//			var symbol string
//			var ktoNum, tokenNum uint64
//			var ktoFee, tokenKtoFee uint64
//			var ktoAddr, tokenAddr types.Address
//			if string(to.SellName) == "kto" {
//				ktoAddr, tokenAddr = *sellAddr, *buyAddr
//				ktoNum, tokenNum = to.SellNumber, to.BuyNumber
//				ktoFee, tokenKtoFee = to.SellFee, to.BuyFee
//				symbol = string(to.BuyName)
//			} else {
//				ktoAddr, tokenAddr = *buyAddr, *sellAddr
//				ktoNum, tokenNum = to.BuyNumber, to.SellNumber
//				ktoFee, tokenKtoFee = to.BuyFee, to.SellFee
//				symbol = string(to.SellName)
//			}
//
//			ktoAvlBalance, ok := avlBalanceResults[sellAddr.String()]
//			if !ok {
//				balance, err := bc.GetBalance(ktoAddr.Bytes())
//				if err != nil {
//					logger.Info("failed get kto bal", zap.Error(err), zap.String("addr", ktoAddr.String()))
//					return nil, err
//				}
//
//				sellFrozenBalance, err := bc.GetFreezeBalance(ktoAddr.Bytes())
//				if err != nil {
//					logger.Info("failed get frozen kto bal", zap.Error(err), zap.String("addr", ktoAddr.String()))
//					return nil, err
//				}
//
//				if util.Uint64SubOverflow(balance, sellFrozenBalance) {
//					logger.Info("sub overflow", zap.Uint64("balance", balance), zap.Uint64("frozen balance", sellFrozenBalance))
//					return nil, errors.New("insufficient balance")
//				}
//
//				ktoAvlBalance = balance - sellFrozenBalance
//			}
//
//			if ktoAvlBalance < ktoNum+ktoFee {
//				logger.Info("ktoAvl<KtoNum+KtoFee", zap.String("addr", ktoAddr.String()), zap.Uint64("ktoAvlBalance", ktoAvlBalance),
//					zap.Uint64("ktoNum", ktoNum), zap.Uint64("ktoFee", ktoFee))
//				continue
//			}
//
//			tokenAvlBal, err := bc.GetAvailableTokenBal(tokenAddr.Bytes(), []byte(symbol))
//			if err != nil {
//				logger.Error("get bal", zap.Error(err), zap.String("token", symbol), zap.String("addr", tokenAddr.String()))
//				continue
//			}
//
//			if tokenAvlBal < tokenNum {
//				logger.Info("tokenAvlBal < tokenNum", zap.String("addr", tokenAddr.String()), zap.Uint64("tokenAvlBal", tokenAvlBal),
//					zap.Uint64("tokenNum", tokenNum))
//				continue
//			}
//
//			tokenKtoAvlBal, ok := avlBalanceResults[tokenAddr.String()]
//			if !ok {
//				bal, err := bc.GetBalance(tokenAddr.Bytes())
//				if err != nil {
//					logger.Info("failed get kto bal", zap.Error(err), zap.String("addr", tokenAddr.String()))
//					continue
//				}
//
//				frozenBal, err := bc.GetFreezeBalance(ktoAddr.Bytes())
//				if err != nil {
//					logger.Info("failed get freeze kto bal", zap.Error(err), zap.String("addr", tokenAddr.String()))
//					return nil, err
//				}
//
//				if util.Uint64SubOverflow(bal, frozenBal) {
//					logger.Info("sub overflow", zap.Uint64("balance", bal), zap.Uint64("frozen balance", frozenBal))
//					return nil, errors.New("insufficient balance")
//				}
//
//				tokenKtoAvlBal = bal - frozenBal
//			}
//
//			if tokenKtoAvlBal+ktoNum < tokenKtoFee {
//				logger.Info("tokenKtoAvlBal+ktoNum < tokenKtoFee", zap.String("addr", tokenAddr.String()), zap.Uint64("tokenKtoAvlBal", tokenKtoAvlBal),
//					zap.Uint64("ktoNum", ktoNum), zap.Uint64("tokenKtoFee", tokenKtoFee))
//				continue
//			}
//
//			avlBalanceResults[ktoAddr.String()] = ktoAvlBalance - ktoNum - ktoFee
//			if bal, ok := avlBalanceResults[tokenAddr.String()]; ok {
//				avlBalanceResults[tokenAddr.String()] = bal + ktoNum - tokenKtoFee
//			} else {
//				avlBalanceResults[tokenAddr.String()] = tokenKtoAvlBal + ktoNum - tokenKtoFee
//			}
//
//			dexBal, ok := avlBalanceResults[DexFeeAddr.String()]
//			if !ok {
//				bal, err := bc.GetBalance(DexFeeAddr.Bytes())
//				if err != nil {
//					logger.Error("failed get dexAddr balance", zap.Error(err), zap.String("dex addr", DexFeeAddr.String()))
//					return nil, errors.New("failed get dex addr balance")
//				}
//				dexBal = bal
//			}
//			avlBalanceResults[DexFeeAddr.String()] = dexBal + ktoFee + tokenKtoFee
//		}
//
//	}
//
//	var buf bytes.Buffer
//	var avlKeys, frzKeys, pckDktoKeys []string
//
//	for key := range avlBalanceResults {
//		avlKeys = append(avlKeys, key)
//	}
//	sort.Strings(avlKeys)
//
//	for key := range frozenBalanceResults {
//		frzKeys = append(frzKeys, key)
//	}
//	sort.Strings(frzKeys)
//
//	for key := range pckDktoResults {
//		pckDktoKeys = append(pckDktoKeys, key)
//	}
//	sort.Strings(pckDktoKeys)
//
//	for _, key := range avlKeys {
//		value := avlBalanceResults[key]
//		addr, _ := types.StringToAddress(key)
//		valBytes := miscellaneous.E64func(value)
//		buf.Write(addr.Bytes())
//		buf.Write(valBytes)
//	}
//
//	for _, key := range frzKeys {
//		value := frozenBalanceResults[key]
//		addr, _ := types.StringToAddress(key)
//		valBytes := miscellaneous.E64func(value)
//		buf.Write(addr.Bytes())
//		buf.Write(valBytes)
//	}
//
//	for _, key := range pckDktoKeys {
//		value := pckDktoResults[key]
//		addr, _ := types.StringToAddress(key)
//		pckBytes := miscellaneous.E64func(value.pck)
//		dktoBytes := miscellaneous.E64func(value.dkto)
//		buf.Write(addr.Bytes())
//		buf.Write(pckBytes)
//		buf.Write(dktoBytes)
//	}
//
//	//TODO：把block中的results删除，换成hash
//	hash := sha256.Sum256(buf.Bytes())
//
//	return hash[:], nil
//}
//
//// CheckResults  重新计算结果，并与结果集对比，相同为true，否则为false
//func (bc *Blockchain) CheckResults(block *block.Block, resultHash, Ds, Cm, qtj []byte) bool {
//	//1、最后一笔交易必须是coinbase交易
//	if !block.Transactions[len(block.Transactions)-1].IsCoinBaseTransaction() {
//		logger.Error("the end is not a coinbase transaction")
//		return false
//	}
//
//	//2、验证leader和follower的结果集是否相同
//	currResultHash, err := bc.CalculationResults(block)
//	if err != nil {
//		logger.Error("failed to calculation results")
//		return false
//	}
//
//	// for _, tx := range block.Transactions {
//	// 	if tx.IsTokenTransaction() {
//	// 		script := parser.Parser([]byte(tx.Script))
//	// 		e, _ := exec.New(bc.cdb, script, string(tx.From.Bytes()))
//	// 		ert := e.Root()
//	// 		if hex.EncodeToString(tx.Root) != hex.EncodeToString(ert) {
//	// 			logger.Error("scrpit", zap.String("root", hex.EncodeToString(tx.Root)), zap.String("ert", hex.EncodeToString(ert)))
//	// 			return false
//	// 		}
//	// 	}
//
//	// 	if !tx.IsCoinBaseTransaction() && !tx.IsFreezeTransaction() {
//	// 		if balance, ok := block.Results[tx.From.String()]; !ok {
//	// 			logger.Info("address is not exist", zap.String("from", tx.From.String()))
//	// 			return false
//	// 		} else if balance != currBlock.Results[tx.From.String()] {
//	// 			logger.Info("balance is not equal", zap.Uint64("curBalnce", balance), zap.Uint64("resBalance", currBlock.Results[tx.From.String()]))
//	// 			return false
//	// 		}
//	// 	}
//
//	// 	if !tx.IsFreezeTransaction() {
//	// 		if balance, ok := block.Results[tx.To.String()]; !ok {
//	// 			logger.Info("address is not exist", zap.String("to", tx.To.String()))
//	// 			return false
//	// 		} else if balance != currBlock.Results[tx.To.String()] {
//	// 			logger.Info("balance is not equal", zap.Uint64("curBalnce", balance), zap.Uint64("resBalance", currBlock.Results[tx.To.String()]))
//	// 			return false
//	// 		}
//	// 	}
//	// }
//	//3、检查各个地址余额
//	log.Debug("length", zap.Int("prev len", len(resultHash)), zap.Int("curr len", len(currResultHash)))
//	if bytes.Compare(resultHash, currResultHash) != 0 {
//		logger.Info("block info", zap.Uint64("height", block.Height))
//		for _, v := range block.Transactions {
//
//			if v.Tag == transaction.TransferTag {
//				tbalance, _ := bc.GetBalance(v.To.Bytes())
//				fbalance, _ := bc.GetBalance(v.From.Bytes())
//				logger.Info("TransferTag address", zap.String("from", string(v.From[:])), zap.String("to", string(v.To[:])),
//					zap.Uint64("tbalance", tbalance), zap.Uint64("fbalance", fbalance), zap.Uint64("amount", v.Amount), zap.Uint64("fee", v.Fee))
//			}
//			if v.Tag == transaction.MinerTag {
//				tbalance, _ := bc.GetBalance(v.To.Bytes())
//				logger.Info("MinerTag address", zap.String("to", string(v.To[:])),
//					zap.Uint64("tbalance", tbalance), zap.Uint64("amount", v.Amount), zap.Uint64("fee", v.Fee))
//			}
//			if v.Tag == transaction.ConvertPckTag || v.Tag == transaction.ConvertKtoTag {
//				fbalance, _ := bc.GetBalance(v.From.Bytes())
//				logger.Info("ConvertPckTag or ConvertKtoTag address", zap.String("from", string(v.From[:])),
//					zap.Uint64("balance", fbalance), zap.Uint64("fee", v.Fee))
//			}
//			if v.Tag == transaction.FreezeTag || v.Tag == transaction.UnfreezeTag {
//				logger.Info("FreezrTag or UnfreeTag address", zap.String("from", string(v.From[:])),
//					zap.Uint64("amount", v.Amount), zap.Uint64("fee", v.Fee))
//			}
//		}
//		logger.Error("hash not equal")
//		return false
//	}
//
//	return true
//}
//
//// GetBlockSection 获取冲lowH到heiH的所有block
//func (bc *Blockchain) GetBlockSection(lowH, heiH uint64) ([]*block.Block, error) {
//	var blocks []*block.Block
//	for i := lowH; i <= heiH; i++ {
//		hash, err := bc.db.Get(append(HeightPrefix, miscellaneous.E64func(i)...))
//		//hash, err := bc.db.Get(miscellaneous.E64func(i), HashKey)
//		if err != nil {
//			logger.Error("Failed to get hash", zap.Error(err), zap.Uint64("height", i))
//			return nil, err
//		}
//		B, err := bc.db.Get(hash)
//		if err != nil {
//			logger.Error("Failed to get block", zap.Error(err), zap.String("hash", string(hash)))
//			return nil, err
//		}
//
//		blcok := &block.Block{}
//		if err := json.Unmarshal(B, blcok); err != nil {
//			logger.Error("Failed to unmarshal block", zap.Error(err), zap.String("hash", string(hash)))
//			return nil, err
//		}
//		blocks = append(blocks, blcok)
//	}
//	return blocks, nil
//}
//
//// GetTokenRoot 获取token的根
//func (bc *Blockchain) GetTokenRoot(address, script string) ([]byte, error) {
//	bc.mu.RLock()
//	defer bc.mu.RUnlock()
//
//	cdbTransaction := bc.cdb.NewTransaction()
//	defer cdbTransaction.Cancel()
//	e, err := exec.New(cdbTransaction, parser.Parser([]byte(script)), address)
//	if err != nil {
//		return nil, err
//	}
//	cdbTransaction.Commit()
//
//	return e.Root(), nil
//}
//
////DeleteBlock delete block
///*
//func (bc *Blockchain) DeleteBlock(height uint64) error {
//	bc.mu.Lock()
//	defer bc.mu.Unlock()
//
//	dbHeight, err := bc.getHeight()
//	if err != nil {
//		logger.Error("failed to get height", zap.Error(err))
//		return err
//	}
//
//	if height > dbHeight {
//		return fmt.Errorf("Wrong height to delete,[%v] should <= current height[%v]", height, dbHeight)
//	}
//
//	for dH := dbHeight; dH >= height; dH-- {
//
//		pckTotal, err := bc.bcgetPckTotal()
//		if err != nil {
//			logger.Error("failed to get total of pck")
//			return err
//		}
//
//		dKtoTotal, err := bc.bcgetDKtoTotal()
//		if err != nil {
//			logger.Error("failed to get total of dkto")
//			return err
//		}
//
//		DBTransaction := bc.db.NewTransaction()
//
//		logger.Info("Start to delete block", zap.Uint64("height", dH))
//		block, err := bc.getBlockByheight(dH)
//		if err != nil {
//			logger.Error("failed to get block", zap.Error(err))
//			return err
//		}
//
//		CDBTransaction := bc.cdb.NewTransaction()
//
//		for i, tx := range block.Transactions {
//			if tx.IsCoinBaseTransaction() {
//				if err = deleteTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(i)); err != nil {
//					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.Uint64("amount", tx.Amount))
//					return err
//				}
//
//				if err := delToAccount(DBTransaction, tx); err != nil {
//					logger.Error("Failed to set account", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.Uint64("amount", tx.Amount))
//					return err
//
//				}
//			} else if tx.IsFreezeTransaction() || tx.IsUnfreezeTransaction() {
//				if err := deleteTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(i)); err != nil {
//					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//					return err
//				}
//
//				if err := deleteTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(i)); err != nil {
//					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//					return err
//				}
//
//				nonce := tx.Nonce
//				if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//					logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				}
//
//				var frozenBalBytes []byte
//				frozenBal, _ := getFreezeBalance(DBTransaction, tx.To.Bytes())
//				if tx.IsFreezeTransaction() {
//					frozenBalBytes = miscellaneous.E64func(tx.Amount - frozenBal)
//				} else {
//					frozenBalBytes = miscellaneous.E64func(frozenBal + tx.Amount)
//				}
//				if err := setFreezeBalance(DBTransaction, tx.To.Bytes(), frozenBalBytes); err != nil {
//					logger.Error("Faile to freeze balance", zap.String("address", tx.To.String()),
//						zap.Uint64("amount", tx.Amount))
//					return err
//				}
//			} else if tx.IsConvertPckTransaction() || tx.IsConvertKtoTransaction() {
//				//待添加
//				if err := deleteTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(i)); err != nil {
//					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//					return err
//				}
//
//				//更新nonce,block中txs必须是有序的
//				nonce := tx.Nonce
//				if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//					logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//					return err
//				}
//
//				if tx.IsConvertPckTransaction() {
//
//					if err := setConvertKto(DBTransaction, tx.From.Bytes(), tx.KtoNum, tx.PckNum); err != nil {
//						logger.Error("Failed to set dkto", zap.Error(err), zap.String("from address", tx.From.String()),
//							zap.Uint64("pck", tx.PckNum), zap.Uint64("dkto", tx.KtoNum))
//						return err
//					}
//
//					if pckTotal < tx.PckNum && dKtoTotal < tx.KtoNum {
//						logger.Error("faile to verify total")
//						return errors.New("uint64 overflow")
//					}
//					pckTotal -= tx.PckNum
//					dKtoTotal -= tx.KtoNum
//				} else {
//					if err := setConvertPck(DBTransaction, tx.From.Bytes(), tx.KtoNum, tx.PckNum); err != nil {
//						logger.Error("Failed to set pck", zap.Error(err), zap.String("from address", tx.From.String()),
//							zap.Uint64("pck", tx.PckNum), zap.Uint64("dkto", tx.KtoNum))
//						return err
//					}
//					if util.Uint64AddOverflow(pckTotal, tx.PckNum) && util.Uint64AddOverflow(dKtoTotal, tx.KtoNum) {
//						logger.Error("faile to verify total")
//						return errors.New("uint64 overflow")
//					}
//					pckTotal += tx.PckNum
//					dKtoTotal += tx.KtoNum
//				}
//			} else {
//				if tx.IsTokenTransaction() {
//					spilt := strings.Split(tx.Script, "\"")
//					if spilt[0] == "transfer " {
//						script := fmt.Sprintf("transfer \"%s\" %s \"%s\"", spilt[1], spilt[2], tx.From.String())
//
//						sc := parser.Parser([]byte(script))
//						e, err := exec.New(CDBTransaction, sc, tx.To.String())
//						if err != nil {
//							logger.Error("Failed to new exec", zap.String("script", script),
//								zap.String("from address", tx.To.String()))
//							tx.Script += " InsufficientBalance"
//						}
//
//						if e != nil {
//							if err = e.Flush(CDBTransaction); err != nil {
//								logger.Error("Failed to flush exec", zap.String("script", tx.Script),
//									zap.String("from address", tx.From.String()))
//								tx.Script += " ExecutionFailed"
//							}
//						}
//					}
//					if err = delMinerFee(DBTransaction, block.Miner.Bytes(), tx.Fee); err != nil {
//						logger.Error("Failed to set fee", zap.Error(err), zap.String("script", tx.Script),
//							zap.String("from address", tx.From.String()), zap.Uint64("fee", tx.Fee))
//						return err
//					}
//				}
//
//				if err := deleteTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(i)); err != nil {
//					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//					return err
//				}
//
//				if err := deleteTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(i)); err != nil {
//					logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//					return err
//				}
//
//				//更新nonce,block中txs必须是有序的
//				nonce := tx.Nonce
//				if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//					logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//					return err
//				}
//
//				//更新余额
//				if err := delAccount(DBTransaction, tx); err != nil {
//					logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//					return err
//				}
//			}
//		}
//		//========
//
//		//高度->哈希
//		hash := block.Hash
//		if err = DBTransaction.Del(append(HeightPrefix, miscellaneous.E64func(block.Height)...)); err != nil {
//			logger.Error("Failed to Del height and hash", zap.Error(err))
//			return err
//		}
//
//		//哈希-> 块
//		if err = DBTransaction.Del(hash); err != nil {
//			logger.Error("Failed to Del block", zap.Error(err))
//			return err
//		}
//		// last
//		DBTransaction.Set(HeightKey, miscellaneous.E64func(dH-1))
//		if err := DBTransaction.Commit(); err != nil {
//			logger.Error("DBTransaction Commit err", zap.Error(err))
//			return err
//		}
//		DBTransaction.Cancel()
//
//		if err := CDBTransaction.Commit(); err != nil {
//			logger.Error("DBTransaction Commit err", zap.Error(err))
//			return err
//		}
//		CDBTransaction.Cancel()
//	}
//
//	logger.Info("End delete")
//	return nil
//
//}
//*/
//
//func deleteTxbyaddrKV(DBTransaction store.Transaction, addr []byte, tx transaction.Transaction, index uint64) error {
//	// txBytes, _ := json.Marshal(tx)
//	// return DBTransaction.Mset(addr, tx.Hash, txBytes)
//	err := DBTransaction.Mdel(addr, tx.Hash)
//	if err != nil {
//		logger.Error("deleteTxbyaddrKV Mdel err ", zap.Error(err))
//		return err
//	}
//
//	if err := DBTransaction.Del(tx.Hash); err != nil {
//		logger.Error("deleteTxbyaddrKV Del err ", zap.Error(err))
//		return err
//	}
//
//	return err
//
//}
//
//func delToAccount(dbTransaction store.Transaction, transaction *transaction.Transaction) error {
//	var balance uint64
//	balanceBytes, err := dbTransaction.Get(transaction.To.Bytes())
//	if err != nil {
//		balance = 0
//	} else {
//		balance, err = miscellaneous.D64func(balanceBytes)
//		if err != nil {
//			return err
//		}
//	}
//
//	newBalanceBytes := miscellaneous.E64func(balance - transaction.Amount)
//	if err := setBalance(dbTransaction, transaction.To.Bytes(), newBalanceBytes); err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func delAccount(DBTransaction store.Transaction, tx *transaction.Transaction) error {
//	from, to := tx.From.Bytes(), tx.To.Bytes()
//
//	fromBalBytes, _ := DBTransaction.Get(from)
//	fromBalance, _ := miscellaneous.D64func(fromBalBytes)
//	if tx.IsTokenTransaction() {
//		fromBalance = fromBalance + tx.Amount + tx.Fee
//	} else {
//		fromBalance += tx.Amount
//	}
//
//	tobalance, err := DBTransaction.Get(to)
//	if err != nil {
//		setBalance(DBTransaction, to, miscellaneous.E64func(0))
//		tobalance = miscellaneous.E64func(0)
//	}
//
//	toBalance, _ := miscellaneous.D64func(tobalance)
//	toBalance -= tx.Amount
//
//	Frombytes := miscellaneous.E64func(fromBalance)
//	Tobytes := miscellaneous.E64func(toBalance)
//
//	if err := setBalance(DBTransaction, from, Frombytes); err != nil {
//		return err
//	}
//	if err := setBalance(DBTransaction, to, Tobytes); err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func delMinerFee(tx store.Transaction, to []byte, amount uint64) error {
//	tobalance, err := tx.Get(to)
//	if err == store.NotExist {
//		tobalance = miscellaneous.E64func(0)
//	} else if err != nil {
//		return err
//	}
//
//	toBalance, _ := miscellaneous.D64func(tobalance)
//	toBalanceBytes := miscellaneous.E64func(toBalance - amount)
//
//	return setBalance(tx, to, toBalanceBytes)
//}
//
////RecoverBlock 向数据库添加新的block数据，minaddr矿工地址
//func (bc *Blockchain) RecoverBlock(block *block.Block, minaddr []byte) error {
//	logger.Info("Start to recover block...", zap.Uint64("height", block.Height))
//	bc.mu.Lock()
//	defer bc.mu.Unlock()
//
//	DBTransaction := bc.db.NewTransaction()
//	defer DBTransaction.Cancel()
//	var err error
//	var height, prevHeight uint64
//	//拿出块高
//	prevHeight, err = bc.getHeight()
//	if err != nil {
//		logger.Error("failed to get height", zap.Error(err))
//		return err
//	}
//
//	height = prevHeight + 1
//	if block.Height != height {
//		return fmt.Errorf("height error:previous height=%d,current height=%d", prevHeight, height)
//	}
//
//	//高度->哈希
//	hash := block.Hash
//	if err = DBTransaction.Set(append(HeightPrefix, miscellaneous.E64func(height)...), hash); err != nil {
//		logger.Error("Failed to set height and hash", zap.Error(err))
//		return err
//	}
//
//	//重置块高
//	DBTransaction.Del(HeightKey)
//	DBTransaction.Set(HeightKey, miscellaneous.E64func(height))
//
//	pckTotal, err := getPckTotal(DBTransaction)
//	if err != nil {
//		logger.Error("failed to get total of pck")
//		return err
//	}
//
//	dKtoTotal, err := getDKtoTotal(DBTransaction)
//	if err != nil {
//		logger.Error("failed to get total of dkto")
//		return err
//	}
//
//	CDBTransaction := bc.cdb.NewTransaction()
//	defer CDBTransaction.Cancel()
//	for index, tx := range block.Transactions {
//		if tx.IsCoinBaseTransaction() {
//			if err = setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			if err := setToAccount(DBTransaction, tx); err != nil {
//				logger.Error("Failed to set account", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.Uint64("amount", tx.Amount))
//				return err
//			}
//		} else if tx.IsFreezeTransaction() || tx.IsUnfreezeTransaction() {
//			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			if err := setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			nonce := tx.Nonce + 1
//			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//			}
//
//			var frozenBalBytes []byte
//			frozenBal, _ := getFreezeBalance(DBTransaction, tx.To.Bytes())
//			if tx.IsFreezeTransaction() {
//				frozenBalBytes = miscellaneous.E64func(tx.Amount + frozenBal)
//				/* 			//投票记录处理
//				vote := NewVote(tx)
//				SetVote(*vote, DBTransaction) */
//			} else {
//				frozenBalBytes = miscellaneous.E64func(frozenBal - tx.Amount)
//			}
//			if err := setFreezeBalance(DBTransaction, tx.To.Bytes(), frozenBalBytes); err != nil {
//				logger.Error("Faile to freeze balance", zap.String("address", tx.To.String()),
//					zap.Uint64("amount", tx.Amount))
//				return err
//			}
//		} else if tx.IsConvertPckTransaction() || tx.IsConvertKtoTransaction() {
//			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			//更新nonce,block中txs必须是有序的
//			nonce := tx.Nonce + 1
//			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			if tx.IsConvertPckTransaction() {
//				if err := setConvertPck(DBTransaction, tx.From.Bytes(), tx.KtoNum, tx.PckNum); err != nil {
//					logger.Error("Failed to set pck", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.Uint64("pck", tx.PckNum), zap.Uint64("dkto", tx.KtoNum))
//					return err
//				}
//
//				if util.Uint64AddOverflow(pckTotal, tx.PckNum) && util.Uint64AddOverflow(dKtoTotal, tx.KtoNum) {
//					logger.Error("faile to verify total")
//					return errors.New("uint64 overflow")
//				}
//				pckTotal += tx.PckNum
//				dKtoTotal += tx.KtoNum
//			} else {
//				if err := setConvertKto(DBTransaction, tx.From.Bytes(), tx.KtoNum, tx.PckNum); err != nil {
//					logger.Error("Failed to set dkto", zap.Error(err), zap.String("from address", tx.From.String()),
//						zap.Uint64("pck", tx.PckNum), zap.Uint64("dkto", tx.KtoNum))
//					return err
//				}
//				if pckTotal < tx.PckNum && dKtoTotal < tx.KtoNum {
//					logger.Error("faile to verify total")
//					return errors.New("uint64 overflow")
//				}
//				pckTotal -= tx.PckNum
//				dKtoTotal -= tx.KtoNum
//
//			}
//		} else {
//			if tx.IsTokenTransaction() {
//				spilt := strings.Split(tx.Script, "\"")
//				if spilt[0] == "transfer " {
//					sc := parser.Parser([]byte(tx.Script))
//					e, err := exec.New(CDBTransaction, sc, tx.From.String())
//					if err != nil {
//						logger.Error("Failed to new exec", zap.String("script", tx.Script),
//							zap.String("from address", tx.From.String()))
//						tx.Script += " InsufficientBalance"
//						block.Transactions[index] = tx
//					}
//
//					if e != nil {
//						if err = e.Flush(CDBTransaction); err != nil {
//							logger.Error("Failed to flush exec", zap.String("script", tx.Script),
//								zap.String("from address", tx.From.String()))
//							tx.Script += " ExecutionFailed"
//							block.Transactions[index] = tx
//						}
//					}
//
//				}
//
//				if err = setMinerFee(DBTransaction, minaddr, tx.Fee); err != nil {
//					logger.Error("Failed to set fee", zap.Error(err), zap.String("script", tx.Script),
//						zap.String("from address", tx.From.String()), zap.Uint64("fee", tx.Fee))
//					return err
//				}
//			}
//
//			if err := setTxbyaddrKV(DBTransaction, tx.From.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			if err := setTxbyaddrKV(DBTransaction, tx.To.Bytes(), *tx, uint64(index)); err != nil {
//				logger.Error("Failed to set transaction", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			//更新nonce,block中txs必须是有序的
//			nonce := tx.Nonce + 1
//			if err := setNonce(DBTransaction, tx.From.Bytes(), miscellaneous.E64func(nonce)); err != nil {
//				logger.Error("Failed to set nonce", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//
//			//更新余额
//			if err := setAccount(DBTransaction, tx); err != nil {
//				logger.Error("Failed to set balance", zap.Error(err), zap.String("from address", tx.From.String()),
//					zap.String("to address", tx.To.String()), zap.Uint64("amount", tx.Amount))
//				return err
//			}
//		}
//
//		// if err := setTxList(DBTransaction, tx); err != nil {
//		// 	logger.Error("Failed to set block data", zap.String("from", tx.From.String()), zap.Uint64("nonce", tx.Nonce))
//		// 	return err
//		// }
//	}
//
//	//哈希-> 块
//	if err = DBTransaction.Set(hash, block.Serialize()); err != nil {
//		logger.Error("Failed to set block", zap.Error(err))
//		return err
//	}
//
//	logger.Info("End recover.")
//	if err := DBTransaction.Commit(); err != nil {
//		return err
//	}
//
//	return CDBTransaction.Commit()
//}
//
//func setConvertPck(tx store.Transaction, from []byte, ktoNum, pckNum uint64) error {
//	var bal, pckBal, dKto uint64
//	// pck
//	pckKey := append([]byte(pckPrefix), from...)
//	pckBalBytes, err := tx.Get(pckKey)
//	if err != nil && err != store.NotExist {
//		return err
//	}
//
//	if err == store.NotExist {
//		pckBal = 0
//	} else {
//		pckBal, _ = miscellaneous.D64func(pckBalBytes)
//	}
//
//	if MAXUINT64-pckBal < pckNum {
//		return errors.New("integer overflow")
//	}
//
//	if err := tx.Set(pckKey, miscellaneous.E64func(pckBal+pckNum)); err != nil {
//		return err
//	}
//
//	// dKto
//	dKtoKey := append([]byte(dKtoPrefix), from...)
//	dKtoBytes, err := tx.Get(dKtoKey)
//	if err != nil && err != store.NotExist {
//		return err
//	}
//
//	if err == store.NotExist {
//		dKto = 0
//	} else {
//		dKto, _ = miscellaneous.D64func(dKtoBytes)
//	}
//
//	if MAXUINT64-dKto < ktoNum {
//		return errors.New("integer overflow")
//	}
//
//	if err := tx.Set(dKtoKey, miscellaneous.E64func(dKto+ktoNum)); err != nil {
//		return err
//	}
//
//	// 余额
//	balBytes, err := tx.Get(from)
//	if err != nil && err != store.NotExist {
//		return err
//	}
//
//	if err == store.NotExist {
//		bal = 0
//	} else {
//		bal, _ = miscellaneous.D64func(balBytes)
//	}
//
//	if bal < ktoNum {
//		fmt.Println(bal)
//		return errors.New("integer overflow")
//	}
//
//	if err := tx.Set(from, miscellaneous.E64func(bal-ktoNum)); err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func setConvertKto(tx store.Transaction, from []byte, ktoNum, pckNum uint64) error {
//	var bal, pckBal, dKto uint64
//	// pck
//	pckKey := append([]byte(pckPrefix), from...)
//	pckBalBytes, err := tx.Get(pckKey)
//	if err != nil && err != store.NotExist {
//		return err
//	}
//
//	if err == store.NotExist {
//		pckBal = 0
//	} else {
//		pckBal, _ = miscellaneous.D64func(pckBalBytes)
//	}
//
//	if pckBal < pckNum {
//		return errors.New("integer overflow")
//	}
//
//	if err := tx.Set(pckKey, miscellaneous.E64func(pckBal-pckNum)); err != nil {
//		return err
//	}
//
//	// dKto
//	dKtoKey := append([]byte(dKtoPrefix), from...)
//	dKtoBytes, err := tx.Get(dKtoKey)
//	if err != nil && err != store.NotExist {
//		return err
//	}
//
//	if err == store.NotExist {
//		dKto = 0
//	} else {
//		dKto, _ = miscellaneous.D64func(dKtoBytes)
//	}
//
//	if dKto < ktoNum {
//		return errors.New("integer overflow")
//	}
//
//	if err := tx.Set(dKtoKey, miscellaneous.E64func(dKto-ktoNum)); err != nil {
//		return err
//	}
//
//	// 余额
//	balBytes, err := tx.Get(from)
//	if err != nil && err != store.NotExist {
//		return err
//	}
//
//	if err == store.NotExist {
//		bal = 0
//	} else {
//		bal, _ = miscellaneous.D64func(balBytes)
//	}
//
//	if MAXUINT64-bal < ktoNum {
//		return errors.New("integer overflow")
//	}
//
//	if err := tx.Set(from, miscellaneous.E64func(bal+ktoNum)); err != nil {
//		return err
//	}
//
//	return nil
//}
//
//// TODO:抵押代币
//// func mortgageTokens(tx store.Transaction, from []byte, tokenPck transaction.TokenPckInfo) error {
//// 	var symbols []string
//// 	for _, v := range tokenPck.Tokens {
//// 		symbols = append(symbols, v.Symbol)
//// 	}
//// 	key := []byte(strings.Join(symbols, "/"))
//// 	mapName := append([]byte("tokenpck"), from...)
//// 	data, err := tx.Mget(mapName, []byte(key))
//// 	if err != nil && err != store.NotExist {
//// 		return err
//// 	}
//
//// 	var prev transaction.TokenPckInfo
//// 	if err != store.NotExist {
//// 		json.Unmarshal(data, &prev)
//// 		if len(prev.Tokens) != len(tokenPck.Tokens) {
//// 			return errors.New("tokens data not equal")
//// 		}
//// 		for i := 0; i < len(symbols); i++ {
//// 			if tokenPck.Tokens[i].Symbol != prev.Tokens[i].Symbol {
//// 				return errors.New("tokens symbol not equal")
//// 			}
//// 		}
//// 	} else {
//
//// 	}
//
//// 	return nil
//// }
//
//func getPckTotal(tx store.Transaction) (uint64, error) {
//	data, err := tx.Get([]byte(PckTotalName))
//	if err != nil && err != store.NotExist {
//		return 0, err
//	}
//
//	if err == store.NotExist {
//		return 0, nil
//	}
//
//	return miscellaneous.D64func(data)
//}
//func (Bc *Blockchain) bcgetPckTotal() (uint64, error) {
//	data, err := Bc.db.Get([]byte(PckTotalName))
//	if err != nil && err != store.NotExist {
//		return 0, err
//	}
//
//	if err == store.NotExist {
//		return 0, nil
//	}
//
//	return miscellaneous.D64func(data)
//}
//
//func getDKtoTotal(tx store.Transaction) (uint64, error) {
//	data, err := tx.Get([]byte(DKtoTotalName))
//	if err != nil && err != store.NotExist {
//		return 0, err
//	}
//
//	if err == store.NotExist {
//		return 0, nil
//	}
//
//	return miscellaneous.D64func(data)
//}
//
//func (Bc *Blockchain) bcgetDKtoTotal() (uint64, error) {
//	data, err := Bc.db.Get([]byte(DKtoTotalName))
//	if err != nil && err != store.NotExist {
//		return 0, err
//	}
//
//	if err == store.NotExist {
//		return 0, nil
//	}
//
//	return miscellaneous.D64func(data)
//}
//
//func (Bc *Blockchain) GetPckTotal() (uint64, error) {
//	tx := Bc.db.NewTransaction()
//	defer tx.Cancel()
//
//	total, err := getPckTotal(tx)
//	if err != nil {
//		return 0, err
//	}
//	return total, tx.Commit()
//}
//
//func (Bc *Blockchain) GetDKtoTotal() (uint64, error) {
//	tx := Bc.db.NewTransaction()
//	defer tx.Cancel()
//	total, err := getDKtoTotal(tx)
//	if err != nil {
//		return 0, err
//	}
//	return total, tx.Commit()
//}
//
//func setPckAndDktoToatal(tx store.Transaction, pckTotal, dKtoTotal uint64) error {
//	if err := tx.Set([]byte(PckTotalName), miscellaneous.E64func(pckTotal)); err != nil {
//		return err
//	}
//	if err := tx.Set([]byte(DKtoTotalName), miscellaneous.E64func(dKtoTotal)); err != nil {
//		return err
//	}
//	return nil
//}
//
//func getPck(tx store.Transaction, addr []byte) (uint64, error) {
//	var num uint64
//	key := append([]byte(pckPrefix), addr...)
//	data, err := tx.Get(key)
//	if err != nil && err != store.NotExist {
//		return 0, err
//	}
//	if err == store.NotExist {
//		num = 0
//	} else {
//		num, _ = miscellaneous.D64func(data)
//	}
//	return num, nil
//}
//
//func getDKto(tx store.Transaction, addr []byte) (uint64, error) {
//	var num uint64
//	key := append([]byte(dKtoPrefix), addr...)
//
//	data, err := tx.Get(key)
//	if err != nil && err != store.NotExist {
//		return 0, err
//	}
//	if err == store.NotExist {
//		num = 0
//	} else {
//		num, _ = miscellaneous.D64func(data)
//	}
//	return num, nil
//}
//
//func (Bc *Blockchain) GetPck(addr []byte) (uint64, error) {
//	tx := Bc.db.NewTransaction()
//	defer tx.Cancel()
//	num, err := getPck(tx, addr)
//	if err != nil {
//		return 0, err
//	}
//	return num, tx.Commit()
//}
//
//func (Bc *Blockchain) GetDKto(addr []byte) (uint64, error) {
//	tx := Bc.db.NewTransaction()
//	defer tx.Cancel()
//
//	num, err := getDKto(tx, addr)
//	if err != nil {
//		return 0, err
//	}
//
//	return num, tx.Commit()
//}
//
//func (Bc *Blockchain) GetTokenDemic(symbol []byte) (uint64, error) {
//
//	Bc.mu.RLock()
//	defer Bc.mu.RUnlock()
//
//	b, err := exec.Precision(Bc.cdb, string(symbol)) // 2，代比， 3，地址
//	if err != nil {
//		return 0, err
//	}
//	return b, nil
//}
//
//func (Bc *Blockchain) GetOrderAllowance(h []byte) (uint64, error) {
//	Bc.mu.RLock()
//	defer Bc.mu.RUnlock()
//	return Bc.orderEngine.GetOrderAllowance(h)
//}
//
//func (bc *Blockchain) GetBindingKtoAddress(ethAddr string) (*types.Address, error) {
//	tx := bc.db.NewTransaction()
//	defer tx.Cancel()
//
//	data, err := tx.Mget(BindingKey, common.HexToAddress(ethAddr).Bytes())
//	if err != nil {
//		if err.Error() != "NotExist" {
//			logger.Error("GetBindingKtoAddress error", zap.String("ethaddr", ethAddr), zap.Error(err))
//		}
//		return nil, err
//	}
//
//	//fmt.Println("End GetBindingKtoAddress>>>>>>>>>>>ktoaddress:", string(data))
//	tx.Commit()
//	return types.BytesToAddress(data)
//}
//
//func (bc *Blockchain) GetBindingEthAddress(ktoAddr types.Address) (string, error) {
//	tx := bc.db.NewTransaction()
//	defer tx.Cancel()
//	defer tx.Commit()
//
//	ks, vs, err := tx.Mkvs(BindingKey)
//	if err != nil {
//		return "", err
//	}
//
//	for id, vl := range vs {
//		if bytes.Equal(ktoAddr.Bytes(), vl) {
//			ethAddr := common.BytesToAddress(ks[id])
//			return ethAddr.String(), nil
//		}
//	}
//
//	return "", errors.New("Not found")
//}
//
//func (bc *Blockchain) CallSmartContract(contractAddr, origin, callInput string) (string, error) {
//	bc.mu.Lock()
//	defer bc.mu.Unlock()
//
//	if callInput[:2] == "0x" {
//		callInput = callInput[2:]
//	}
//	fmt.Println("CallSmartContract contractAddr====", contractAddr, "origin====", origin, "callInput====", common.Hex2Bytes(callInput))
//
//	if len(callInput) > 0 && len(contractAddr) > 0 {
//		snapshotId := bc.evm.GetSnapshot()
//
//		dump1 := bc.evm.RawDump()
//		ethAddrs := make(map[common.Address]*big.Int)
//		//this for sets balance to the corresponding eth address [ethAddrs] in contract.
//		for addr, _ := range dump1.Accounts {
//			kAddr, err := bc.GetBindingKtoAddress(addr.String())
//			if err != nil {
//				continue
//			}
//			DBTransaction := bc.db.NewTransaction()
//			defer DBTransaction.Cancel()
//			fromBalBytes, _ := DBTransaction.Get(kAddr.Bytes())
//			kBalance, _ := miscellaneous.D64func(fromBalBytes)
//
//			//fmt.Println("check set balance1", addr, kBalance)
//
//			bc.evm.SetBalance(addr, big.NewInt(0).SetUint64(kBalance))
//		}
//
//		defer func() {
//			//this for cleans Previously set[ethAddrs]
//			bc.evm.RevertToSnapshot(snapshotId)
//
//			for addr, _ := range ethAddrs {
//				delete(ethAddrs, addr)
//			}
//		}()
//
//		ret, gasLeft, err := bc.evm.Call(common.HexToAddress(contractAddr), common.HexToAddress(origin), common.Hex2Bytes(callInput))
//
//		logger.Info("blockchain CallSmartContract", zap.String("contract addr", contractAddr), zap.String("ret:", common.Bytes2Hex(ret)), zap.Uint64("gasLeft", gasLeft), zap.Error(err))
//		return common.Bytes2Hex(ret), err
//	}
//
//	logger.Error("faile to Call", zap.Error(fmt.Errorf("wrong input[%v] or contractaddr[%v]", callInput, contractAddr)))
//	return "", fmt.Errorf("wrong input[%v] or contractaddr[%v]", callInput, contractAddr)
//}
//
//func (bc *Blockchain) GetCode(contractAddr string) []byte {
//	code := bc.evm.GetCode(common.HexToAddress(contractAddr))
//	//fmt.Println("addr,getcode", common.HexToAddress(contractAddr), common.Bytes2Hex(code))
//	return code
//}
//
//func (bc *Blockchain) GetLogs(hash common.Hash) []*evmtypes.Log {
//	return bc.evm.GetLogs(hash)
//}
//
//func (bc *Blockchain) Logs() []*evmtypes.Log {
//	return bc.evm.Logs()
//}
//
//func (bc *Blockchain) HandleContract(block *block.Block, DBTransaction store.Transaction, tx *transaction.Transaction, index int) error {
//	eth_tx, err := transaction.DecodeData(tx.EthData)
//
//	deci := new(big.Int).SetUint64(ETHDECIMAL)
//	value := eth_tx.Value().Div(eth_tx.Value(), deci)
//	bc.evm.SetValue(value, eth_tx.GasPrice())
//
//	bc.evm.Prepare(common.BytesToHash(tx.Hash), common.BytesToHash(block.Hash), index)
//
//	snapshotId := bc.evm.GetSnapshot()
//	ethAddrs := make(map[common.Address]*big.Int)
//	dump1 := bc.evm.RawDump()
//	//this for sets balance to the corresponding eth address [ethAddrs] in contract.
//	for addr, _ := range dump1.Accounts {
//		kAddr, err := bc.GetBindingKtoAddress(addr.String())
//		if err != nil {
//			continue
//		}
//
//		DBTransaction := bc.db.NewTransaction()
//		defer DBTransaction.Cancel()
//		fromBalBytes, _ := DBTransaction.Get(kAddr.Bytes())
//		kBalance, _ := miscellaneous.D64func(fromBalBytes)
//
//		//fmt.Println("set balance to evm:", addr, acc.Balance)
//		bc.evm.SetBalance(addr, big.NewInt(0).SetUint64(kBalance))
//		ethAddrs[addr] = big.NewInt(0).SetUint64(kBalance)
//
//		// bl := bc.evm.GetBalance(addr)
//		// fmt.Println("HandleContract check set balance1", addr, bl)
//	}
//
//	var callErr error
//	defer func() {
//		dump2 := bc.evm.RawDump()
//		//do withdraw
//		if callErr == nil {
//			for addr, acc := range dump2.Accounts {
//				if balance, ok := big.NewInt(0).SetString(acc.Balance, 10); ok {
//					kAddr, err := bc.GetBindingKtoAddress(addr.String())
//					if err != nil {
//						continue
//					}
//					//fmt.Println("set balance to kto ===================balance:", balance)
//					fromBalance := balance.Uint64()
//
//					if _, ok := ethAddrs[addr]; !ok {
//						DBTransaction := bc.db.NewTransaction()
//						defer DBTransaction.Cancel()
//
//						fromBalBytes, _ := DBTransaction.Get(kAddr.Bytes())
//						kBalance, _ := miscellaneous.D64func(fromBalBytes)
//
//						fromBalance = kBalance + balance.Uint64()
//						//fmt.Println("set balance to kto (kBalance + balance.Uint64()):", fromBalance)
//					}
//					Frombytes := miscellaneous.E64func(fromBalance)
//					if err := setBalance(DBTransaction, kAddr.Bytes(), Frombytes); err != nil {
//						logger.Error("tx call setBalance error!!!:", zap.Error(err))
//						return
//					}
//
//					bc.evm.SetBalance(addr, big.NewInt(0))
//
//					// bl := bc.evm.GetBalance(addr)
//					// fmt.Println("HandleContract check balance2", addr, bl)
//				}
//			}
//		} else {
//			bc.evm.RevertToSnapshot(snapshotId)
//		}
//
//		for k, _ := range ethAddrs {
//			delete(ethAddrs, k)
//		}
//
//	}()
//
//	switch tx.EvmC.Operation {
//	case "create", "Create":
//		//fmt.Println("create contract ++++++++++++++++,code length", len(tx.EvmC.CreateCode), tx.EvmC.Origin, "nounce:", bc.evm.GetNonce(tx.EvmC.Origin))
//		ret, contractAddr, gasLeft, callErr := bc.evm.Create(tx.EvmC.CreateCode, tx.EvmC.Origin)
//		if callErr != nil {
//			logger.Error("faile to create contract", zap.String("name", tx.EvmC.ContractName), zap.Uint64("gasLeft", gasLeft), zap.Error(callErr))
//			tx.EvmC.Ret = common.Bytes2Hex(ret)
//			tx.EvmC.Status = false
//			return callErr
//		}
//
//		_, err = bc.evm.StateCommit(false, tx.BlockNumber, tx.EvmC.ContractName, contractAddr, common.BytesToHash(tx.Hash))
//		if err != nil {
//			logger.Error("faile to commit state", zap.Error(err))
//			return err
//		}
//		tx.EvmC.ContractAddr = contractAddr
//		tx.EvmC.Status = true
//
//		logger.Info("Create successfully", zap.String("contract address:", contractAddr.Hex()), zap.Uint64("gasLeft", gasLeft))
//	case "call", "Call":
//		//fmt.Println("tx call contract ===================", tx.EvmC.ContractAddr, tx.EvmC.Origin, common.Bytes2Hex(tx.EvmC.CallInput), "value", eth_tx.Value(), "gasprice", eth_tx.GasPrice())
//
//		ret, gasLeft, callErr := bc.evm.Call(tx.EvmC.ContractAddr, tx.EvmC.Origin, tx.EvmC.CallInput)
//		if callErr != nil {
//			logger.Error("faile to Call", zap.Uint64("gasLeft", gasLeft), zap.Error(callErr))
//			tx.EvmC.Ret = common.Bytes2Hex(ret)
//			tx.EvmC.Status = false
//			return callErr
//		}
//
//		_, err = bc.evm.StateCommit(false, tx.BlockNumber, "", common.Address{}, common.BytesToHash(tx.Hash))
//		if err != nil {
//			logger.Error("faile to commit state", zap.Error(err))
//			return err
//		}
//
//		logger.Info("Call successfully", zap.String("contract address:", tx.EvmC.ContractAddr.Hex()), zap.String("ret:", common.Bytes2Hex(ret)), zap.Uint64("gasLeft", gasLeft))
//		tx.EvmC.Ret = common.Bytes2Hex(ret)
//		tx.EvmC.Status = true
//
//		strInput := common.Bytes2Hex(tx.EvmC.CallInput)
//		fmt.Println("check strInput>>>>>", strInput)
//		if strings.Contains(strInput, TRS) || strings.Contains(strInput, TRSF) || strings.Contains(strInput, APPR) {
//			tl := len(strInput)
//			strV := strInput[tl-64:]
//			fmt.Println("check strV>>>>>", strV)
//
//			if v, ok := big.NewInt(0).SetString(strV, 16); ok {
//				fmt.Println("check value>>>>>", v)
//				ret, _, err := bc.evm.Call(tx.EvmC.ContractAddr, tx.EvmC.Origin, common.Hex2Bytes(DECI)) //ret, err := bc.CallSmartContract(tx.EvmC.ContractAddr.Hex(), tx.EvmC.Origin.Hex(), DECI)
//				fmt.Println("check ret,err>>>>>", ret, err)
//				if err == nil {
//					deci, ok := big.NewInt(0).SetString(common.Bytes2Hex(ret), 16)
//					fmt.Println("check deci>>>>>", deci)
//					if ok && deci.Int64() > 0 {
//						decimal := big.NewInt(0).SetUint64(uint64(math.Pow10(int(deci.Int64()))))
//						tx.EvmC.Value = v.Div(v, decimal).Uint64()
//						fmt.Println("check value>>>>>", tx.EvmC.Value)
//					}
//				}
//			}
//		}
//	}
//	return nil
//}
//
//func (bc *Blockchain) GetStorageAt(addr, hash string) common.Hash {
//	return bc.evm.GetStorageAt(common.HexToAddress(addr), common.HexToHash(hash))
//}
