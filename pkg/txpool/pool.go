package txpool

import (
	"fmt"
	"sync"
	"time"

	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/blockchain"
	_ "github.com/korthochain/korthochain/pkg/crypto/sigs/ed25519"
	_ "github.com/korthochain/korthochain/pkg/crypto/sigs/secp"
	"github.com/korthochain/korthochain/pkg/transaction"
	"github.com/korthochain/korthochain/pkg/util/math"
	"go.uber.org/zap"
)

const (
	PendingLimit = 100
	poolCap      = 10000
	timesub      = 100
)

// TransactionPool is a temporary storage pool for unchained transactions.
// Transactions are sorted by GasCap and Nonce in the pool and waiting to be
// uploaded to the chain.
// type TransactionPool interface {
// 	push(*transaction.SignedTransaction) error
// 	Pending() ([]*transaction.SignedTransaction, error)
// 	IBlockchain
// }

type IBlockchain interface {
	GetNonce(address.Address) (uint64, error)
	GetAvailableBalance(address.Address) (uint64, error)
	GetAllFreezeBalance(address address.Address) (uint64, error)
	GetSingleFreezeBalance(address.Address, address.Address) (uint64, error)
}

// Pool is a temporary storage pool for unchained transactions.
// Transactions are sorted by GasCap and Nonce in the pool and waiting to be
// uploaded to the chain.
type Pool struct {
	qlock sync.Mutex
	q     *orderlyQueue

	bc         IBlockchain
	logger     *zap.Logger
	pendingBuf []transaction.SignedTransaction
}

// NewPool Create transaction pool
func NewPool(cfg Config) (*Pool, error) {
	if cfg.BlockChain == nil {
		return nil, fmt.Errorf("bc cannot be empty")
	}

	p := &Pool{
		bc:         cfg.BlockChain,
		q:          newQueue(),
		pendingBuf: make([]transaction.SignedTransaction, PendingLimit, PendingLimit),
	}

	if cfg.Logger != nil {
		p.logger = cfg.Logger
	} else {
		p.logger = zap.NewNop()
	}

	return p, nil
}

// Add to add a new transaction to the pool, outdated transactions
// will return an error
func (p *Pool) Add(st *transaction.SignedTransaction) error {
	if st.Type == transaction.TransferTransaction {
		if len(st.Input) > 0 {
			return fmt.Errorf("Unsupported Token transaction currently,input: %v", string(st.Input))
		}
	}
	if st.GasLimit*st.GasPrice < blockchain.MINGASLIMIT || st.GasLimit*st.GasPrice > blockchain.MAXGASLIMIT {
		return fmt.Errorf("gas is too small or too big,gas limit:%d gas price:%d", st.GasLimit, st.GasPrice)
	}

	if err := st.VerifySign(); err != nil {
		return err
	}

	p.qlock.Lock()
	defer p.qlock.Unlock()
	p.logger.Debug("add tx", zap.String("transaction", st.String()))

	return p.add(st)
}

func (p *Pool) AddList(stList []transaction.SignedTransaction) []error {
	p.qlock.Lock()
	defer p.qlock.Unlock()

	var eList []error
	for i := 0; i < len(stList); i++ {
		if err := stList[i].VerifySign(); err != nil {
			err = fmt.Errorf("transaction:%s,error:%v", stList[i].String(), err)
			eList = append(eList, err)
			continue
		}

		if err := p.add(&stList[i]); err != nil {
			err = fmt.Errorf("transaction:%s,error:%v", stList[i].String(), err)
			eList = append(eList, err)
		}
	}

	return eList
}

func (p *Pool) add(st *transaction.SignedTransaction) error {
	if p.q.len() >= poolCap {
		return fmt.Errorf("pool is full,please try again later")
	}

	// Check if the nonce of the transaction is required
	if err := p.geCallerNonce(st.Caller(), st.GetNonce()); err != nil {
		return err
	}

	p.q.push(*st)
	return nil
}

func (p *Pool) geCallerNonce(caller address.Address, nonce uint64) error {
	lastNonce, _ := p.bc.GetNonce(caller)
	if nonce < lastNonce {
		return fmt.Errorf("transaction.nonce must be greater than the chain nonce : transaction nonce(%d) chain nonce (%d)", nonce, lastNonce)
	}
	return nil
}

type traderInfo struct {
	availableBalance uint64
	lockBalance      uint64
	nextNonce        uint64
	sigleLockBalance uint64
	sigleExist       bool
}

// String
func (t *traderInfo) String() string {
	return fmt.Sprintf("avalible balance %d,lock balance %d,next nonce:%d",
		t.availableBalance, t.lockBalance, t.nextNonce)
}

// Pending returns the transaction that can be packaged
func (p *Pool) Pending() ([]transaction.SignedTransaction, error) {
	p.qlock.Lock()
	defer p.qlock.Unlock()

	traderBuffer := make(map[string]traderInfo)
	i, l := 0, p.q.len()

	var badTxIdxs []int
	a := 0
	for ; a < PendingLimit && i < l; i++ {
		idx := l - 1 - i
		st := p.q.stList[idx]
		if n := p.compareNonce(traderBuffer, st.Caller(), st.GetNonce()); n < 0 {
			traderInfo, err := p.getTraderInfo(traderBuffer, st.Caller(), nil)
			if err != nil {
				p.logger.Error("getTraderInfo", zap.String("address", st.Caller().String()), zap.Error(err))
				continue
			}
			p.logger.Error("compare nonce", zap.String("trader", traderInfo.String()),
				zap.String("transaction", st.Transaction.String()))
			badTxIdxs = append(badTxIdxs, idx)
			continue
		} else if n > 0 {
			traderInfo, err := p.getTraderInfo(traderBuffer, st.Caller(), nil)
			if err != nil {
				p.logger.Error("getTraderInfo", zap.String("address", st.Caller().String()), zap.Error(err))
				continue
			}
			p.logger.Error("compare nonce", zap.String("trader", traderInfo.String()),
				zap.String("transaction", st.Transaction.String()))
			continue
		}

		if err := p.transactionPreCalculated(traderBuffer, st.Transaction); err != nil {
			p.logger.Error("pre-calculated", zap.Error(err), zap.String("transaction", st.Transaction.String()))
			badTxIdxs = append(badTxIdxs, idx)
			continue
		}
		p.pendingBuf[a] = st
		a++
		p.logger.Debug("success pending", zap.String("transaction", st.Transaction.String()))
	}

	for _, idx := range badTxIdxs {
		p.q.remove(idx)
	}

	return p.pendingBuf[:a], nil
}

func (p *Pool) transactionPreCalculated(traderBuf map[string]traderInfo, tx transaction.Transaction) error {
	switch tx.Type {
	case transaction.TransferTransaction:
		return p.transferTransactionPreCalculated(traderBuf, tx)
	case transaction.LockTransaction:
		return p.lockTransactionPreCalculated(traderBuf, tx)
	case transaction.UnlockTransaction:
		return p.unlockTransactionPreCalculated(traderBuf, tx)
	case transaction.PledgeTrasnaction, transaction.EvmContractTransaction, transaction.EvmKtoTransaction:
		return p.pledgeTrasnactionPreCalculated(traderBuf, tx)
	case transaction.PledgeBreakTransaction:
		return p.redeemTrasnactionPreCalculated(traderBuf, tx)
	// 	TODO: Remaining types
	default:
		return fmt.Errorf("unknown transaction type:%d", tx.Type)
	}
}

func (p *Pool) transferTransactionPreCalculated(traderBuf map[string]traderInfo, tx transaction.Transaction) error {
	caller, err := p.getTraderInfo(traderBuf, tx.Caller(), nil)
	if err != nil {
		return err
	}

	receiver, err := p.getTraderInfo(traderBuf, tx.Receiver(), nil)
	if err != nil {
		return err
	}

	amountAndGas, err := math.AddUint64Overflow(tx.Amount, tx.GasCap())
	if err != nil {
		return fmt.Errorf(err.Error()+": amount(%d) + gas cap(%d)", tx.Amount, tx.GasCap())
	}

	callerBal, err := math.SubUint64Overflow(caller.availableBalance, amountAndGas)
	if err != nil {
		return fmt.Errorf(err.Error()+":caller avalible balance(%d) - amount and gas(%d)",
			caller.availableBalance, amountAndGas)
	}

	receiverBal, err := math.AddUint64Overflow(receiver.availableBalance, tx.Amount)
	if err != nil {
		return fmt.Errorf(err.Error()+": receiver avalible balance(%d) + amount(%d)",
			receiver.availableBalance, tx.Amount)
	}

	caller.nextNonce++
	caller.availableBalance = callerBal
	receiver.availableBalance = receiverBal

	traderBuf[tx.Caller().String()] = caller
	traderBuf[tx.Receiver().String()] = receiver
	return nil
}

func (p *Pool) lockTransactionPreCalculated(traderBuf map[string]traderInfo, tx transaction.Transaction) error {
	caller, err := p.getTraderInfo(traderBuf, tx.Caller(), nil)
	if err != nil {
		return err
	}

	if caller.availableBalance < tx.Amount {
		return fmt.Errorf("avaliable balance(%d) < lock amount(%d)", caller.availableBalance, tx.Amount)
	}

	lockBal, err := math.AddUint64Overflow(caller.lockBalance, tx.Amount)
	if err != nil {
		return fmt.Errorf(err.Error()+": locked balance(%d) + lock amount(%d)", caller.lockBalance, tx.Amount)
	}

	avlBal, err := math.SubUint64Overflow(caller.availableBalance, tx.Amount)
	if err != nil {
		return fmt.Errorf(err.Error()+": avaliable balance(%d) - lock amount(%d)", caller.availableBalance, tx.Amount)
	}

	avlBal, err = math.SubUint64Overflow(avlBal, tx.GasCap())
	if err != nil {
		return fmt.Errorf(err.Error()+": avaliable balance(%d) - lock amount(%d) - gas(%d)", caller.availableBalance, tx.Amount, tx.GasCap())
	}

	caller.nextNonce++
	caller.lockBalance = lockBal
	caller.availableBalance = avlBal

	traderBuf[tx.Caller().String()] = caller
	return nil
}

func (p *Pool) unlockTransactionPreCalculated(traderBuf map[string]traderInfo, tx transaction.Transaction) error {
	caller, err := p.getTraderInfo(traderBuf, tx.Caller(), &tx.To)
	if err != nil {
		return err
	}

	if caller.sigleLockBalance < tx.Amount {
		return fmt.Errorf("single locked balance(%d) < lock amount(%d)", caller.lockBalance, tx.Amount)
	}

	lockBal, err := math.SubUint64Overflow(caller.lockBalance, tx.Amount)
	if err != nil {
		return fmt.Errorf(err.Error()+": locked balance(%d) - lock amount(%d)", caller.lockBalance, tx.Amount)
	}

	singleBal, err := math.SubUint64Overflow(caller.sigleLockBalance, tx.Amount)
	if err != nil {
		return fmt.Errorf(err.Error()+": single locked balance(%d) - lock amount(%d)", caller.lockBalance, tx.Amount)
	}

	avlBal, err := math.SubUint64Overflow(caller.availableBalance, tx.GasCap())
	if err != nil {
		return fmt.Errorf(err.Error()+": avalible balance(%d) + lock amount(%d) - gas(%d)", caller.availableBalance, tx.Amount, tx.GasCap())
	}

	avlBal, err = math.AddUint64Overflow(avlBal, tx.Amount)
	if err != nil {
		return fmt.Errorf(err.Error()+": avaliable balance(%d) + lock amount(%d)", caller.availableBalance, tx.Amount)
	}

	caller.nextNonce++
	caller.lockBalance = lockBal
	caller.availableBalance = avlBal
	caller.sigleLockBalance = singleBal

	traderBuf[tx.Caller().String()] = caller
	return nil
}

func (p *Pool) pledgeTrasnactionPreCalculated(traderBuf map[string]traderInfo, tx transaction.Transaction) error {
	caller, err := p.getTraderInfo(traderBuf, tx.Caller(), nil)
	if err != nil {
		return err
	}

	if caller.availableBalance < tx.Amount {
		return fmt.Errorf("available balance(%d) < lock amount(%d)", caller.availableBalance, tx.Amount)
	}

	avlBalance, err := math.SubUint64Overflow(caller.availableBalance, tx.Amount)
	if err != nil {
		return fmt.Errorf(err.Error()+": availbale balance(%d) - pledge amount(%d)", caller.availableBalance, tx.Amount)
	}

	avlBalance, err = math.SubUint64Overflow(avlBalance, tx.GasCap())
	if err != nil {
		return fmt.Errorf(err.Error()+": availbale balance(%d) - gas fee(%d)", avlBalance, tx.GasCap())
	}

	caller.nextNonce++
	caller.availableBalance = avlBalance
	traderBuf[tx.Caller().String()] = caller

	return nil
}

func (p *Pool) redeemTrasnactionPreCalculated(traderBuf map[string]traderInfo, tx transaction.Transaction) error {
	if tx.Amount != 0 {
		return fmt.Errorf("tx.Amount expect 0,tx.Amount[%v]", tx.Amount)
	}

	caller, err := p.getTraderInfo(traderBuf, tx.Caller(), nil)
	if err != nil {
		return err
	}

	if caller.availableBalance < tx.Amount {
		return fmt.Errorf("available balance(%d) < lock amount(%d)", caller.availableBalance, tx.Amount)
	}

	avlBalance, err := math.SubUint64Overflow(caller.availableBalance, tx.GasCap())
	if err != nil {
		return fmt.Errorf(err.Error()+": availbale balance(%d) - gas fee(%d)", avlBalance, tx.GasCap())
	}

	caller.nextNonce++
	caller.availableBalance = avlBalance
	traderBuf[tx.Caller().String()] = caller
	return nil
}

func (p *Pool) getTraderInfo(traderBuf map[string]traderInfo, trader address.Address, receiver *address.Address) (traderInfo, error) {
	traderInfo, ok := traderBuf[trader.String()]
	if ok && receiver == nil {
		return traderInfo, nil
	}

	var err error
	if !ok {
		traderInfo.availableBalance, err = p.bc.GetAvailableBalance(trader)
		if err != nil {
			return traderInfo, err
		}
		traderInfo.lockBalance, err = p.bc.GetAllFreezeBalance(trader)
		if err != nil {
			return traderInfo, err
		}
		traderInfo.nextNonce, err = p.bc.GetNonce(trader)
		if err != nil {
			return traderInfo, err
		}
	}

	if receiver != nil && !traderInfo.sigleExist {
		traderInfo.sigleLockBalance, err = p.bc.GetSingleFreezeBalance(trader, *receiver)
		if err != nil {
			return traderInfo, err
		}
		traderInfo.sigleExist = true
	}

	traderBuf[trader.String()] = traderInfo

	return traderInfo, nil
}

// if nonce > nextnonce return 1
//    nonce = nextnonce return 0
//    nonce < nextnonce return -1
func (p *Pool) compareNonce(traderBuf map[string]traderInfo, trader address.Address, nonce uint64) int {
	traderInfo, err := p.getTraderInfo(traderBuf, trader, nil)
	if err != nil {
		p.logger.Error("compareNonce", zap.Error(err))
	}

	if nonce < traderInfo.nextNonce {
		return -1
	} else if nonce > traderInfo.nextNonce {
		return 1
	}

	return 0
}

// FilterTransaction filter incoming transactions
func (p *Pool) FilterTransaction(stList []transaction.SignedTransaction) {
	p.qlock.Lock()
	defer p.qlock.Unlock()

	for _, st := range stList {
		rstList, ok := p.q.rstBuffer[st.Caller().String()]
		if !ok {
			continue
		}

		for _, rst := range rstList {
			if st.GetNonce() == rst.nonce {
				p.q.remove(rst.idx)
			}
		}
	}

	p.cacheOutSignedTransaction()
}

// TransctionCacheOut  Eliminate transactions in the transaction pool
func (p *Pool) cacheOutSignedTransaction() error {
	for k, v := range p.q.timeBuffer {
		if v > time.Now().Unix()-3*timesub {
			continue
		}

		delete(p.q.timeBuffer, k)

		caller, nonce, err := parseKey(k)
		if err != nil {
			p.logger.Error("CacheOutSignedTransaction,parseKey", zap.Error(err))
			continue
		}

		addr, err := address.NewAddrFromString(caller)
		if err != nil {
			p.logger.Error("CacheOutSignedTransaction,NewAddrFromString", zap.Error(err))
			continue
		}

		idx, ok := p.findSignedTransactionIdx(addr, nonce)
		if !ok {
			continue
		}

		p.q.remove(idx)
		p.logger.Debug("remove tx", zap.String("addr", addr.String()), zap.Uint64("nonce", nonce))
	}

	for p.q.len() > poolCap/3*2 {
		p.logger.Debug("remove tx", zap.String("addr", p.q.stList[0].Caller().String()), zap.Uint64("nonce", p.q.stList[0].GetNonce()))
		p.q.remove(0)
	}

	return nil
}

func (p *Pool) findSignedTransactionIdx(addr address.Address, nonce uint64) (int, bool) {
	list, ok := p.q.rstBuffer[addr.String()]
	if !ok {
		return -1, false
	}

	for _, rst := range list {
		if rst.nonce == nonce {
			if p.q.len() <= rst.idx {
				break
			}

			return rst.idx, true
		}
	}
	return -1, false
}

func (p *Pool) FindSignedTransaction(addr address.Address, nonce uint64) (*transaction.SignedTransaction, bool) {
	p.qlock.Lock()
	defer p.qlock.Unlock()

	list, ok := p.q.rstBuffer[addr.String()]
	if !ok {
		return nil, false
	}

	for _, rst := range list {
		if rst.nonce == nonce {
			if p.q.len() <= rst.idx {
				break
			}

			return &p.q.stList[rst.idx], true
		}
	}

	return nil, false
}

func (p *Pool) CopySignedTransactions() []transaction.SignedTransaction {
	p.qlock.Lock()
	defer p.qlock.Unlock()

	stc := make([]transaction.SignedTransaction, p.q.len())

	copy(stc, p.q.stList)
	return stc
}

func (p *Pool) GetTxByHash(hash string) (*transaction.SignedTransaction, error) {
	p.qlock.Lock()
	defer p.qlock.Unlock()

	addrStr, nonce, err := p.q.getAddrAndNonceByHash(hash)
	if err != nil {
		return nil, err
	}
	addr, err := address.NewAddrFromString(addrStr)
	if err != nil {
		return nil, err
	}

	idx, ok := p.findSignedTransactionIdx(addr, nonce)
	if !ok {
		return nil, fmt.Errorf("not exist")
	}

	if idx > len(p.q.stList) || p.q.stList[idx].HashToString() != hash {
		return nil, fmt.Errorf("not exist")
	}

	return &p.q.stList[idx], nil
}
