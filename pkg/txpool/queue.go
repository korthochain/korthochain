package txpool

import (
	"fmt"
	"time"

	"github.com/korthochain/korthochain/pkg/transaction"
)

//var _ txheap = new(orderlyQueue)

type orderlyQueue struct {
	stList    []transaction.SignedTransaction
	rstBuffer map[string][]receivedTransaction
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
		stList:    []transaction.SignedTransaction{},
		rstBuffer: make(map[string][]receivedTransaction),
	}
}

func (q *orderlyQueue) len() int {
	return len(q.stList)
}
func (q *orderlyQueue) push(st transaction.SignedTransaction) {

	if rstList, ok := q.rstBuffer[st.Caller().String()]; ok {
		for _, rst := range rstList {
			if rst.nonce == st.Nonce() {
				if rst.price >= st.GasCap() {
					return
				}

				q.update(st.Caller().String(), rst.nonce, withPrice(st.GasCap()), withTimestamp(time.Now().Second()))
				q.stList[rst.idx] = st
				for i := rst.idx; i < q.len(); i++ {
					q.down(i, q.len()-1)
				}
				return
			}
		}
	}

	q.stList = append(q.stList, st)
	q.addRstBuffer(st.Caller().String(), receivedTransaction{st.Nonce(), option{q.len() - 1, st.GasCap(), time.Now().Second()}})
	q.up(q.len() - 1)
}

func (q *orderlyQueue) pop() transaction.SignedTransaction {
	st := q.stList[q.len()-1]
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
	q.rmRecivedTransaction(q.stList[idx].Caller().String(), idx)

	for i := idx + 1; i < q.len(); i++ {
		q.update(q.stList[i].Caller().String(), q.stList[i].Nonce(), withIdx(i-1))
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

		q.update(q.stList[i].Caller().String(), q.stList[i].Nonce(), withIdx(i+1))
		q.stList[i+1] = q.stList[i]
	}

	q.update(value.Caller().String(), value.Nonce(), withIdx(i+1))
	q.stList[i+1] = value
}

func (q *orderlyQueue) down(i, j int) {
	value := q.stList[i]
	i += 1
	for ; i <= j; i++ {
		if q.less(i, value) {
			break
		}

		q.update(q.stList[i].Caller().String(), q.stList[i].Nonce(), withIdx(i-1))
		q.stList[i-1] = q.stList[i]
	}

	q.update(value.Caller().String(), value.Nonce(), withIdx(i-1))
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
		return y.Nonce() > x.Nonce()
	}

	rstList := q.rstBuffer[y.Caller().String()]
	if len(rstList) == 1 {
		return y.GasCap() <= x.GasCap()
	}

	for _, rst := range rstList {
		if rst.nonce < y.Nonce() && rst.idx < i {
			return true
		}
	}

	return y.GasCap() <= x.GasCap()
}
