package transaction

import (
	"testing"

	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/crypto"
	"github.com/korthochain/korthochain/pkg/crypto/sigs"

	_ "github.com/korthochain/korthochain/pkg/crypto/sigs/ed25519"

	"github.com/stretchr/testify/assert"
)

func TestCodec(t *testing.T) {
	assert := assert.New(t)
	var max = ^uint64(0)
	fromPriv, _ := sigs.Generate(crypto.ED25519)
	fromPub, _ := sigs.ToPublic(crypto.ED25519, fromPriv)
	from, _ := address.NewEd25519Addr(fromPub)
	to, _ := address.NewFromBytes([]byte{215, 107, 11, 147, 201, 41, 120, 88, 133, 22, 237, 60, 113, 122, 93, 210, 7, 56, 133, 215, 192, 220, 83, 0, 54, 122, 173, 194, 70, 161, 154, 139})
	iput := make([]byte, 20)
	tx := &Transaction{
		Version: 1,
		From:    from,
		To:      to,
		Amount:  max,
		Nonce:   max,
		Type:    1,

		GasLimit:  max,
		GasFeeCap: max,
		GasPrice:  max,
		Input:     iput,
	}

	siganature, err := sigs.Sign(crypto.ED25519, fromPriv, tx.SignHash())

	assert.NoError(err)

	st := SignedTransaction{
		Transaction: *tx,
		Signature:   *siganature,
	}

	buf, err := st.Serialize()
	assert.NoError(err)

	maybeTx, err := DeserializeSignaturedTransaction(buf)
	assert.NoError(err)

	assert.Equal(st, *maybeTx)
}
