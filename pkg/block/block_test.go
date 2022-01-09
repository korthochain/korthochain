package block

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/korthochain/korthochain/pkg/address"
	_ "github.com/korthochain/korthochain/pkg/crypto/sigs/ed25519"
	"github.com/korthochain/korthochain/pkg/transaction"
)

func TestBlockCodec(t *testing.T) {
	assert := assert.New(t)
	// from, err := address.NewAddrFromString("otKBvWNrk4Dgi2yeHf2ZEdmHxXadBtdEqAVAmwH1qBbePP2")
	// assert.NoError(err)

	// to, err := address.NewAddrFromString("otKBvWNrk4Dgi2yeHf2ZEdmHxXadBtdEqAVAmwH1qBbePP3")
	// assert.NoError(err)

	minaddr, err := address.NewAddrFromString("otKBvWNrk4Dgi2yeHf2ZEdmHxXadBtdEqAVAmwH1qBbePP4")
	assert.NoError(err)

	// tmp := &transaction.Transaction{
	// 	Version:  1,
	// 	Type:     transaction.LockTransaction,
	// 	Nonce:    1,
	// 	From:     from,
	// 	To:       to,
	// 	Amount:   1000000,
	// 	GasLimit: 10,
	// 	GasPrice: 10,
	// }
	// priv, _ := base58.Decode("2DiXYY3moSdNtwCzCfTMqZgpkX2EAwQyKMZe3BFPc9MvKpx5sDcgDkHS6kGzqnJTaUJULg3VNBVxb8MaFBHHLRLQ")
	// siganature, err := sigs.Sign(crypto.TypeED25519, priv, tmp.SignHash())
	assert.NoError(err)

	// stx := transaction.SignedTransaction{
	// 	Transaction: *tmp,
	// 	Signature:   *siganature,
	// }

	// ft := transaction.FinishedTransaction{
	// 	SignedTransaction: stx,
	// 	GasUsed:           10,
	// }

	b := &Block{
		Height:   1,
		PrevHash: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Hash:     []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		//Transactions: []*transaction.FinishedTransaction{&ft},
		Transactions:     []*transaction.FinishedTransaction{},
		Root:             []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SnapRoot:         []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Version:          1,
		Timestamp:        1111111,
		Miner:            minaddr,
		Difficulty:       big.NewInt(1000),
		GlobalDifficulty: big.NewInt(1111),
		Nonce:            1,
		GasLimit:         1,
		GasUsed:          2,
	}
	data, err := b.Serialize()
	assert.NoError(err)

	maybe, err := Deserialize(data)
	assert.NoError(err)
	t.Log("data len:", len(data))
	assert.Equal(maybe, b)
}
