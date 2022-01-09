package txpool

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/transaction"
	"go.uber.org/zap"
)

type orderlyQueue struct {
	stList     []transaction.SignedTransaction
	rstBuffer  map[string][]receivedTransaction
	timeBuffer map[string]int64
	hashBuffer map[string]string
}

type receivedTransaction struct {
	nonce uint64
	option
}

type option struct {
	idx       int
	price     uint64
	timestamp int
}

func newQueue() *orderlyQueue {
	return &orderlyQueue{
		stList:     []transaction.SignedTransaction{},
		rstBuffer:  make(map[string][]receivedTransaction),
		timeBuffer: make(map[string]int64),
		hashBuffer: make(map[string]string),
	}
}

func (q *orderlyQueue) len() int {
	return len(q.stList)
}
func (q *orderlyQueue) push(st transaction.SignedTransaction) {

	if rstList, ok := q.rstBuffer[st.Caller().String()]; ok {
		for _, rst := range rstList {
			if rst.nonce == st.GetNonce() {
				if rst.price >= st.GasCap() {
					return
				}

				q.update(st.Caller().String(), rst.nonce, withPrice(st.GasCap()), withTimestamp(time.Now().Second()))
				delete(q.hashBuffer, q.stList[rst.idx].HashToString())
				q.stList[rst.idx] = st
				q.hashBuffer[st.HashToString()] = fmt.Sprint(st.Caller().String() + "|" + strconv.FormatUint(st.Nonce, 10))
				for i := rst.idx; i < q.len(); i++ {
					q.down(i, q.len()-1)
				}
				q.timeBuffer[timeKey(st.Caller().String(), st.GetNonce())] = time.Now().Unix()
				return
			}
		}
	}

	q.stList = append(q.stList, st)
	q.hashBuffer[st.HashToString()] = fmt.Sprint(st.Caller().String() + "|" + strconv.FormatUint(st.Nonce, 10))
	q.addRstBuffer(st.Caller().String(), receivedTransaction{st.GetNonce(), option{q.len() - 1, st.GasCap(), time.Now().Second()}})
	logger.Info("add  ", zap.String("hash", st.HashToString()), zap.String("tx", st.String()))
	if _, ok := q.timeBuffer[timeKey(st.Caller().String(), st.GetNonce())]; !ok {
		q.timeBuffer[timeKey(st.Caller().String(), st.GetNonce())] = time.Now().Unix()
	}

	q.up(q.len() - 1)
}

func (q *orderlyQueue) pop() transaction.SignedTransaction {
	st := q.stList[q.len()-1]
	delete(q.hashBuffer, st.HashToString())
	q.rmRecivedTransaction(st.Caller().String(), q.len()-1)
	q.stList = q.stList[:q.len()-1]

	return st
}

func (q *orderlyQueue) getIndex(caller string, nonce uint64) (receivedTransaction, error) {
	rstList := q.rstBuffer[caller]
	for x := 0; x < len(rstList); x++ {
		if rstList[x].nonce == nonce {
			return rstList[x], nil
		}
	}
	return receivedTransaction{}, fmt.Errorf("not exist")
}

func (q *orderlyQueue) remove(idx int) {
	if idx < 0 || idx >= q.len() {
		return
	}

	logger.Info("remove:", zap.String("hash", q.stList[idx].HashToString()), zap.String("tx", q.stList[idx].String()))
	q.rmRecivedTransaction(q.stList[idx].Caller().String(), idx)
	delete(q.hashBuffer, q.stList[idx].HashToString())

	for i := idx + 1; i < q.len(); i++ {
		q.update(q.stList[i].Caller().String(), q.stList[i].GetNonce(), withIdx(i-1))
		q.stList[i-1] = q.stList[i]
	}
	q.stList = q.stList[:q.len()-1]

	for i := idx - 1; i >= 0; i-- {
		q.down(i, q.len()-1)
	}
}

func (q *orderlyQueue) up(j int) {
	if j < 0 {
		return
	}

	i := j - 1
	value := q.stList[j]

	for ; i >= 0; i-- {
		if !q.less(i, value) {
			break
		}

		q.update(q.stList[i].Caller().String(), q.stList[i].GetNonce(), withIdx(i+1))
		q.stList[i+1] = q.stList[i]
	}

	q.update(value.Caller().String(), value.GetNonce(), withIdx(i+1))
	q.stList[i+1] = value
}

func (q *orderlyQueue) down(i, j int) {
	value := q.stList[i]
	i += 1
	for ; i <= j; i++ {
		if q.less(i, value) {
			break
		}

		q.update(q.stList[i].Caller().String(), q.stList[i].GetNonce(), withIdx(i-1))
		q.stList[i-1] = q.stList[i]
	}

	q.update(value.Caller().String(), value.GetNonce(), withIdx(i-1))
	q.stList[i-1] = value
}

func (q *orderlyQueue) addRstBuffer(caller string, rst receivedTransaction) {
	if _, ok := q.rstBuffer[caller]; !ok {
		q.rstBuffer[caller] = []receivedTransaction{rst}
		return
	}
	q.rstBuffer[caller] = append(q.rstBuffer[caller], rst)
}

type modOption func(option *option)

func withPrice(price uint64) modOption {
	return func(o *option) {
		o.price = price
	}
}

func withIdx(idx int) modOption {
	return func(o *option) {
		o.idx = idx
	}
}

func withTimestamp(timestamp int) modOption {
	return func(o *option) {
		o.timestamp = timestamp
	}
}

func (q *orderlyQueue) update(caller string, nonce uint64, modOptions ...modOption) {
	rstList := q.rstBuffer[caller]
	for x := 0; x < len(rstList); x++ {
		if rstList[x].nonce == nonce {
			for _, fn := range modOptions {
				fn(&rstList[x].option)
			}
			break
		}
	}
	q.rstBuffer[caller] = rstList
}

func (q *orderlyQueue) rmRecivedTransaction(caller string, idx int) {
	rstList := q.rstBuffer[caller]

	for i := 0; i < len(rstList); i++ {
		if rstList[i].idx == idx {
			rstList = append(rstList[:i], rstList[i+1:]...)
			break
		}
	}

	if len(rstList) == 0 {
		delete(q.rstBuffer, caller)
	} else {
		q.rstBuffer[caller] = rstList
	}
}

// y <=  x ture
// y > x false
func (q *orderlyQueue) less(i int, y transaction.SignedTransaction) bool {
	x := q.stList[i]
	if y.Caller().String() == x.Caller().String() {
		return y.GetNonce() > x.GetNonce()
	}

	rstList := q.rstBuffer[y.Caller().String()]
	if len(rstList) == 1 {
		return y.GasCap() <= x.GasCap()
	}

	for _, rst := range rstList {
		if rst.nonce < y.GetNonce() && rst.idx < i {
			return true
		}
	}

	return y.GasCap() <= x.GasCap()
}

func timeKey(caller string, nonce uint64) string {
	return fmt.Sprintf("%s_%d", caller, nonce)
}

func parseKey(key string) (string, uint64, error) {
	list := strings.Split(key, "_")
	if len(list) != 2 {
		return "", 0, fmt.Errorf("invalid time key")
	}

	nonce, err := strconv.ParseUint(list[1], 10, 64)
	if err != nil {
		return "", 0, err
	}
	return list[0], nonce, nil
}

func (q *orderlyQueue) HashExist(hash string) bool {
	_, ok := q.hashBuffer[hash]
	return ok
}

func (q *orderlyQueue) getAddrAndNonceByHash(hash string) (string, uint64, error) {
	key, ok := q.hashBuffer[hash]
	if !ok {
		return "", 0, fmt.Errorf("not exist")
	}

	n := strings.Index(key, "|")
	if n < 0 {
		return "", 0, fmt.Errorf("")
	}

	addr := key[:n]
	nonce, err := strconv.ParseUint(key[n+1:], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("")
	}
	return addr, nonce, nil
}
