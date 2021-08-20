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

func (h *orderlyQueue) len() int {
	return len(h.stList)
}
func (h *orderlyQueue) push(st transaction.SignedTransaction) {

	if rstList, ok := h.rstBuffer[st.Caller().String()]; ok {
		for _, rst := range rstList {
			if rst.nonce == st.Nonce() {
				if rst.price >= st.GasCap() {
					return
				}

				h.update(st.Caller().String(), rst.nonce, withPrice(st.GasCap()), withTimestamp(time.Now().Second()))
				h.stList[rst.idx] = st
				for i := rst.idx; i < h.len(); i++ {
					h.down(i, h.len()-1)
				}
				return
			}
		}
	}

	h.stList = append(h.stList, st)
	h.addRstBuffer(st.Caller().String(), receivedTransaction{st.Nonce(), option{h.len() - 1, st.GasCap(), time.Now().Second()}})
	h.up(h.len() - 1)
}

func (h *orderlyQueue) pop() transaction.SignedTransaction {
	st := h.stList[h.len()-1]
	h.rmRecivedTransaction(st.Caller().String(), h.len()-1)
	h.stList = h.stList[:h.len()-1]

	return st
}

func (h *orderlyQueue) getIndex(caller string, nonce uint64) (receivedTransaction, error) {
	rstList := h.rstBuffer[caller]
	for x := 0; x < len(rstList); x++ {
		if rstList[x].nonce == nonce {
			return rstList[x], nil
		}
	}
	return receivedTransaction{}, fmt.Errorf("not exist")
}

func (h *orderlyQueue) remove(idx int) {
	if idx < 0 || idx >= h.len() {
		return
	}
	h.rmRecivedTransaction(h.stList[idx].Caller().String(), idx)

	for i := idx + 1; i < h.len(); i++ {
		h.update(h.stList[i].Caller().String(), h.stList[i].Nonce(), withIdx(i-1))
		h.stList[i-1] = h.stList[i]
	}
	h.stList = h.stList[:h.len()-1]

	for i := idx - 1; i >= 0; i-- {
		h.down(i, h.len()-1)
	}
}

func (h *orderlyQueue) up(j int) {
	if j < 0 {
		return
	}

	i := j - 1
	value := h.stList[j]

	for ; i >= 0; i-- {
		if !h.less(i, value) {
			break
		}

		h.update(h.stList[i].Caller().String(), h.stList[i].Nonce(), withIdx(i+1))
		h.stList[i+1] = h.stList[i]
	}

	h.update(value.Caller().String(), value.Nonce(), withIdx(i+1))
	h.stList[i+1] = value
}

func (h *orderlyQueue) down(i, j int) {
	value := h.stList[i]
	i += 1
	for ; i <= j; i++ {
		if h.less(i, value) {
			break
		}

		h.update(h.stList[i].Caller().String(), h.stList[i].Nonce(), withIdx(i-1))
		h.stList[i-1] = h.stList[i]
	}

	h.update(value.Caller().String(), value.Nonce(), withIdx(i-1))
	h.stList[i-1] = value
}

func (h *orderlyQueue) addRstBuffer(caller string, rst receivedTransaction) {
	if _, ok := h.rstBuffer[caller]; !ok {
		h.rstBuffer[caller] = []receivedTransaction{rst}
		return
	}
	h.rstBuffer[caller] = append(h.rstBuffer[caller], rst)
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

func (h *orderlyQueue) update(caller string, nonce uint64, modOptions ...modOption) {
	rstList := h.rstBuffer[caller]
	for x := 0; x < len(rstList); x++ {
		if rstList[x].nonce == nonce {
			for _, fn := range modOptions {
				fn(&rstList[x].option)
			}
			break
		}
	}
	h.rstBuffer[caller] = rstList
}

func (h *orderlyQueue) rmRecivedTransaction(caller string, idx int) {
	rstList := h.rstBuffer[caller]

	for i := 0; i < len(rstList); i++ {
		if rstList[i].idx == idx {
			rstList = append(rstList[:i], rstList[i+1:]...)
			break
		}
	}

	if len(rstList) == 0 {
		delete(h.rstBuffer, caller)
	} else {
		h.rstBuffer[caller] = rstList
	}
}

// y <=  x ture
// y > x false
func (h *orderlyQueue) less(i int, y transaction.SignedTransaction) bool {
	x := h.stList[i]
	if y.Caller().String() == x.Caller().String() {
		return y.Nonce() > x.Nonce()
	}

	rstList := h.rstBuffer[y.Caller().String()]
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
