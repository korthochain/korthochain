package txpool

import (
	"testing"

	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/transaction"
	"github.com/stretchr/testify/assert"
)

func TestStHeappush(t *testing.T) {
	assert := assert.New(t)
	h := newQueue()

	addr1, _ := address.NewAddrFromString("otKG7B6mFNNHHLNqLCbFMhz2bbwPJuRv4fMbthXJtcaCAJq")
	addr2, _ := address.NewAddrFromString("otKFVuKvsDLUb5zWMutcroqs8WiocjgmWuF55WE4GYvfhvA")
	addr3, _ := address.NewAddrFromString("otK4uXfcTtYYRfFprzxuxzAqqgjx2nTdKUw1WdzybQ2ukn6")
	srcList := []transaction.SignedTransaction{
		0:  {Transaction: transaction.Transaction{From: addr1, Nonce: 3, GasFeeCap: 1, GasPrice: 3}},
		1:  {Transaction: transaction.Transaction{From: addr2, Nonce: 1, GasFeeCap: 1, GasPrice: 8}},
		2:  {Transaction: transaction.Transaction{From: addr3, Nonce: 2, GasFeeCap: 1, GasPrice: 3}},
		3:  {Transaction: transaction.Transaction{From: addr1, Nonce: 1, GasFeeCap: 1, GasPrice: 5}},
		4:  {Transaction: transaction.Transaction{From: addr2, Nonce: 2, GasFeeCap: 1, GasPrice: 6}},
		5:  {Transaction: transaction.Transaction{From: addr3, Nonce: 1, GasFeeCap: 1, GasPrice: 1}},
		6:  {Transaction: transaction.Transaction{From: addr1, Nonce: 1, GasFeeCap: 1, GasPrice: 7}},
		7:  {Transaction: transaction.Transaction{From: addr2, Nonce: 3, GasFeeCap: 1, GasPrice: 4}},
		8:  {Transaction: transaction.Transaction{From: addr3, Nonce: 3, GasFeeCap: 1, GasPrice: 9}},
		9:  {Transaction: transaction.Transaction{From: addr3, Nonce: 4, GasFeeCap: 1, GasPrice: 1}},
		10: {Transaction: transaction.Transaction{From: addr3, Nonce: 1, GasFeeCap: 1, GasPrice: 10}},
	}

	//sort
	{

		inputIdx := []int{0, 1, 2, 3, 4, 5}
		for _, idx := range inputIdx {
			h.push(srcList[idx])
		}

		outputIdx := []int{1, 4, 3, 0, 5, 2}
		for _, idx := range outputIdx {
			maybe := h.pop()
			assert.Equal(srcList[idx].GasCap(), maybe.GasCap())
		}
	}

	// update
	{
		inputIdx := []int{0, 1, 2, 3, 4, 5, 6}
		for _, idx := range inputIdx {
			h.push(srcList[idx])
		}

		outputIdx := []int{1, 6, 4, 0, 5, 2}
		for _, idx := range outputIdx {
			maybe := h.pop()
			assert.Equal(srcList[idx].GasCap(), maybe.GasCap())
		}
	}

	// remove
	{
		inputIdx := []int{0, 1, 2, 3, 4, 5, 6}
		for _, idx := range inputIdx {
			h.push(srcList[idx])
		}

		h.remove(2)
		outputIdx := []int{1, 6, 4, 5, 2}
		for _, idx := range outputIdx {
			maybe := h.pop()
			assert.Equal(srcList[idx].GasCap(), maybe.GasCap())
		}
	}

	// update and remove
	{
		inputIdx := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		for _, idx := range inputIdx {
			h.push(srcList[idx])
		}

		h.remove(2)

		outputIdx := []int{10, 8, 1, 6, 4, 7, 0, 9}
		for _, idx := range outputIdx {
			maybe := h.pop()
			assert.Equal(srcList[idx].GasCap(), maybe.GasCap())
		}
	}
}
