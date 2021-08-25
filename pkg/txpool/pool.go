package txpool

import (
	"fmt"
	"sync"
	"time"

	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/transaction"
	"github.com/korthochain/korthochain/pkg/util/math"
	"go.uber.org/zap"
)

const (
	pendingLimit = 100
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
	GetNonce(address.Address) uint64
	GetLockBalance(address.Address) uint64
	GetAvalibleBalance(address.Address) uint64
}

// Pool is a temporary storage pool for unchained transactions.
// Transactions are sorted by GasCap and Nonce in the pool and waiting to be
// uploaded to the chain.
type Pool struct {
	qlock      sync.Mutex
	q          *orderlyQueue
	pendingBuf []transaction.SignedTransaction
	bc         IBlockchain

	logger *zap.Logger
}

// NewPool Create transaction pool
func NewPool(cfg Config) (*Pool, error) {
	if cfg.BlockChain == nil {
		return nil, fmt.Errorf("bc cannot be empty")
	}

	p := &Pool{
		bc:         cfg.BlockChain,
		q:          newQueue(),
		pendingBuf: make([]transaction.SignedTransaction, pendingLimit),
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
	p.qlock.Lock()
	defer p.qlock.Unlock()

	if p.q.len() >= poolCap {
		return fmt.Errorf("pool is full,please try again later")
	}

	// Check if the nonce of the transaction is required
	if err := p.geCallerNonce(st.Caller(), st.Nonce()); err != nil {
		return err
	}

	p.q.push(*st)
	return nil
}

func (p *Pool) geCallerNonce(caller address.Address, nonce uint64) error {
	lastNonce := p.bc.GetNonce(caller)
	if nonce < lastNonce {
		return fmt.Errorf("transaction.nonce must be greater than the chain nonce : transaction nonce(%d) < chain nonce (%d)", lastNonce, nonce)
	}
	return nil
}

type traderInfo struct {
	availableBalance uint64
	lockBalance      uint64
	nextNonce        uint64
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
	unreadyBuffer := make([]transaction.SignedTransaction, 0)

	i := 0
	for ; i < pendingLimit && i < p.q.len(); i++ {
		st := p.q.pop()

		if n := p.compareNonce(traderBuffer, st.Caller(), st.Nonce()); n < 0 {
			i--
			traderInfo := p.getTraderInfo(traderBuffer, st.Caller())
			p.logger.Error("compare nonce", zap.String("trader", traderInfo.String()), zap.String("transaction", st.Transaction.String()))
			continue
		} else if n > 0 {
			i--
			unreadyBuffer = append(unreadyBuffer, st)
			traderInfo := p.getTraderInfo(traderBuffer, st.Caller())
			p.logger.Info("compare nonce", zap.String("trader", traderInfo.String()), zap.String("transaction", st.Transaction.String()))
			continue
		}

		if err := p.transactionPreCalculated(traderBuffer, st.Transaction); err != nil {
			i--
			p.logger.Error("pre-calculated", zap.Error(err), zap.String("transaction", st.Transaction.String()))
			continue
		}
		p.pendingBuf[i] = st
	}

	p.pendingBuf = p.pendingBuf[:i]

	for _, unready := range unreadyBuffer {
		p.q.push(unready)
	}

	return p.pendingBuf, nil
}

func (p *Pool) transactionPreCalculated(traderBuf map[string]traderInfo, tx transaction.Transaction) error {
	switch tx.Type {
	case transaction.TransferTransaction:
		return p.transferTransactionPreCalculated(traderBuf, tx)
	case transaction.LockTransaction:
		return p.lockTransactionPreCalculated(traderBuf, tx)
	case transaction.UnlockTransaction:
		return p.unlockTransactionPreCalculated(traderBuf, tx)

	// 	TODO: Remaining types
	default:
		return fmt.Errorf("unknown transaction type:%d", tx.Type)
	}
}

func (p *Pool) transferTransactionPreCalculated(traderBuf map[string]traderInfo, tx transaction.Transaction) error {
	caller := p.getTraderInfo(traderBuf, tx.Caller())
	receiver := p.getTraderInfo(traderBuf, tx.Receiver())

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

	caller.availableBalance = callerBal
	receiver.availableBalance = receiverBal

	traderBuf[tx.Caller().String()] = caller
	traderBuf[tx.Receiver().String()] = receiver
	return nil
}

func (p *Pool) lockTransactionPreCalculated(traderBuf map[string]traderInfo, tx transaction.Transaction) error {
	caller := p.getTraderInfo(traderBuf, tx.Caller())

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

	caller.lockBalance = lockBal
	caller.availableBalance = avlBal

	traderBuf[tx.Caller().String()] = caller
	return nil
}

func (p *Pool) unlockTransactionPreCalculated(traderBuf map[string]traderInfo, tx transaction.Transaction) error {
	caller := p.getTraderInfo(traderBuf, tx.Caller())

	if caller.lockBalance < tx.Amount {
		return fmt.Errorf("locked balance(%d) < lock amount(%d)", caller.lockBalance, tx.Amount)
	}

	lockBal, err := math.SubUint64Overflow(caller.lockBalance, tx.Amount)
	if err != nil {
		return fmt.Errorf(err.Error()+": locked balance(%d) + lock amount(%d)", caller.lockBalance, tx.Amount)
	}

	avlBal, err := math.AddUint64Overflow(caller.availableBalance, tx.Amount)
	if err != nil {
		return fmt.Errorf(err.Error()+": avaliable balance(%d) + lock amount(%d)", caller.availableBalance, tx.Amount)
	}

	avlBal, err = math.SubUint64Overflow(avlBal, tx.GasCap())
	if err != nil {
		return fmt.Errorf(err.Error()+": avalible balance(%d) + lock amount(%d) - gas(%d)", caller.availableBalance, tx.Amount, tx.GasCap())
	}

	caller.lockBalance = lockBal
	caller.availableBalance = avlBal

	traderBuf[tx.Caller().String()] = caller
	return nil
}

func (p *Pool) getTraderInfo(traderBuf map[string]traderInfo, trader address.Address) traderInfo {
	traderInfo, ok := traderBuf[trader.String()]
	if ok {
		return traderInfo
	}

	traderInfo.availableBalance = p.bc.GetAvalibleBalance(trader)
	traderInfo.lockBalance = p.bc.GetLockBalance(trader)
	traderInfo.nextNonce = p.bc.GetNonce(trader)
	traderBuf[trader.String()] = traderInfo

	return traderInfo
}

// if nonce > nextnonce return 1
//    nonce = nextnonce return 0
//    nonce < nextnonce return -1
func (p *Pool) compareNonce(traderBuf map[string]traderInfo, trader address.Address, nonce uint64) int {
	traderInfo := p.getTraderInfo(traderBuf, trader)

	if nonce < traderInfo.nextNonce {
		return -1
	} else if nonce > traderInfo.nextNonce {
		return 1
	}

	traderInfo.nextNonce += 1
	traderBuf[trader.String()] = traderInfo

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
			if st.Nonce() == rst.nonce {
				p.q.remove(rst.idx)
			}
		}
	}
}

// TransctionCacheOut  Eliminate transactions in the transaction pool
func (p *Pool) CacheOutSignedTransaction() error {
	p.qlock.Lock()
	defer p.qlock.Unlock()

	if p.q.len() > poolCap/2 {
		for i := p.q.len() / 2; i < p.q.len(); i++ {
			st := p.q.stList[i]
			rst, err := p.q.getIndex(st.Caller().String(), st.Nonce())
			if err != nil {
				return err
			}

			if rst.timestamp > time.Now().Second()-timesub {
				break
			}

			p.q.remove(i)
		}
	}

	return nil
}
