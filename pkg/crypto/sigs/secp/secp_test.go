package secp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

var testmsg = hexutil.MustDecode("0xce0677bb30baa8cf067c88db9811f4333d131bf8bcf12fe7065d211dce971008")
var testsig = hexutil.MustDecode("0x90f27b8b488db00b00606796d2987f6a5f59ae62ea05effe84fef5b8b0e549984a691139ad57a3f0b906637673aa2f63d1f55cb1a69199d4009eea23ceaddc9301")
var testpubkey = hexutil.MustDecode("0x04e32df42865e97135acfb65f3bae71bdc86f4d49150ad6a440b6f15878109880a0a2b2667f7e725ceea70c673093bf67663e0312623c8e091b13cf2c0f11ef652")

func TestEth(t *testing.T) {
	assert := assert.New(t)
	signer := new(Secp256k1Signer)
	privBytes, err := signer.Generate()
	assert.NoError(err)

	priv := crypto.ToECDSAUnsafe(privBytes)
	t.Log(hex.EncodeToString(crypto.FromECDSAPub(&priv.PublicKey)))

	msg := sha256.Sum256([]byte("kortho"))
	signature, err := signer.Sign(privBytes, msg[:])
	assert.NoError(err)

	pubkey, err := signer.ToPublic(privBytes)
	assert.NoError(err)

	pub, err := crypto.UnmarshalPubkey(pubkey)
	assert.NoError(err)

	addr := crypto.PubkeyToAddress(*pub)
	{
		err := signer.Verify(signature, append([]byte{0}, addr.Bytes()...), msg[:])
		assert.NoError(err)
	}

	{
		tmp := make([]byte, len(msg))
		rand.Read(tmp)
		err := signer.Verify(signature, pubkey, tmp)
		assert.Error(err)
	}
}
