package address

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/korthochain/korthochain/pkg/crypto"
	"github.com/korthochain/korthochain/pkg/crypto/sigs"
	_ "github.com/korthochain/korthochain/pkg/crypto/sigs/ed25519"
	_ "github.com/korthochain/korthochain/pkg/crypto/sigs/secp"

	"github.com/stretchr/testify/assert"
)

func TestED25519Address(t *testing.T) {
	CurrentNetWork = Testnet

	assert := assert.New(t)
	priv, err := sigs.Generate(crypto.TypeSecp256k1)
	assert.NoError(err)

	t.Log("priv:", hex.EncodeToString(priv))

	pub, err := sigs.ToPublic(crypto.TypeSecp256k1, priv)
	assert.NoError(err)

	addr, err := NewSecp256k1Addr(pub)
	assert.NoError(err)
	t.Log("addr:", addr)

	str, err := encode(CurrentNetWork, addr)
	assert.NoError(err)

	maybe, err := decode(str)
	assert.NoError(err)

	assert.Equal(addr, maybe)
}

func TestVectorsED25519Address(t *testing.T) {
	testCases := []struct {
		input    []byte
		testAddr string
		mainAddr string
	}{
		{[]byte{224, 116, 53, 72, 47, 178, 42, 166, 231, 150, 128, 178, 181, 240, 198, 37, 204, 23, 29, 220, 79, 134, 85, 155, 225, 181, 80, 76, 255, 153, 249, 54},
			"otKG7B6mFNNHHLNqLCbFMhz2bbwPJuRv4fMbthXJtcaCAJq",
			"KtoG7B6mFNNHHLNqLCbFMhz2bbwPJuRv4fMbthXJtcaCAJq",
		},
		{
			[]byte{215, 107, 11, 147, 201, 41, 120, 88, 133, 22, 237, 60, 113, 122, 93, 210, 7, 56, 133, 215, 192, 220, 83, 0, 54, 122, 173, 194, 70, 161, 154, 139},
			"otKFVuKvsDLUb5zWMutcroqs8WiocjgmWuF55WE4GYvfhvA",
			"KtoFVuKvsDLUb5zWMutcroqs8WiocjgmWuF55WE4GYvfhvA",
		},
		{
			[]byte{58, 7, 239, 202, 148, 198, 174, 121, 6, 224, 129, 2, 194, 115, 15, 200, 239, 221, 106, 80, 206, 77, 27, 250, 84, 76, 112, 60, 123, 254, 67, 83},
			"otK4uXfcTtYYRfFprzxuxzAqqgjx2nTdKUw1WdzybQ2ukn6",
			"Kto4uXfcTtYYRfFprzxuxzAqqgjx2nTdKUw1WdzybQ2ukn6",
		},
		{
			[]byte{33, 52, 79, 191, 63, 76, 90, 18, 1, 171, 98, 172, 122, 253, 179, 155, 115, 108, 211, 47, 130, 66, 90, 186, 141, 8, 241, 134, 248, 208, 163, 232},
			"otK3EcigM1uS3xZ95t6EDnvrKtPegaVMLm1SkLenxQhiWFZ",
			"Kto3EcigM1uS3xZ95t6EDnvrKtPegaVMLm1SkLenxQhiWFZ",
		},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("testing ed25519 address: %s(testnet),%s(mainnet)", tc.testAddr, tc.mainAddr)
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			//testnet
			// Round trip encoding and decoding from string
			CurrentNetWork = Testnet

			addr, err := NewAddrFromString(tc.testAddr)
			assert.NoError(err)
			assert.Equal(tc.input, addr.Payload())

			maybeAddr, err := NewAddrFromString(tc.testAddr)
			assert.NoError(err)
			assert.Equal(tc.input, maybeAddr.Payload())

			// Round trip encoding and decoding from bytes

			maybeAddrBytes, err := NewFromBytes(maybeAddr.Bytes())
			assert.NoError(err)
			assert.Equal(tc.input, maybeAddrBytes.Payload())

			//testnet
			// Round trip encoding and decoding from string
			CurrentNetWork = Mainnet

			mianAddr, err := NewAddrFromString(tc.mainAddr)
			assert.NoError(err)
			assert.Equal(tc.input, mianAddr.Payload())

			maybeMainAddr, err := NewAddrFromString(tc.mainAddr)
			assert.NoError(err)
			assert.Equal(tc.input, maybeMainAddr.Payload())

			// Round trip encoding and decoding from bytes
			maybeMainAddrBytes, err := NewFromBytes(maybeMainAddr.Bytes())
			assert.NoError(err)
			assert.Equal(tc.input, maybeMainAddrBytes.Payload())
		})
	}

}

func TestTransactionCBOR(t *testing.T) {
	assert := assert.New(t)
	a, _ := NewFromBytes([]byte{224, 116, 53, 72, 47, 178, 42, 166, 231, 150, 128, 178, 181, 240, 198, 37, 204, 23, 29, 220, 79, 134, 85, 155, 225, 181, 80, 76, 255, 153, 249, 54})
	buf := bytes.NewBuffer(nil)

	err := a.MarshalCBOR(buf)
	assert.NoError(err)

	maybeAddr := new(Address)
	err = maybeAddr.UnmarshalCBOR(buf)
	assert.NoError(err)

	assert.Equal(a, *maybeAddr)
}

func TestGenesisAddress(t *testing.T) {
	assert := assert.New(t)
	priv, err := sigs.Generate(crypto.TypeSecp256k1)
	assert.NoError(err)

	pub, err := sigs.ToPublic(crypto.TypeSecp256k1, priv)
	assert.NoError(err)

	addr, err := NewSecp256k1Addr(pub)
	assert.NoError(err)

	t.Log("addr:", addr.String())
	t.Log("length:", len(addr.String()))
	genAddr := "otK00000000000000000000000000000000000000000"
	addr, err = NewAddrFromString(genAddr)
	assert.NoError(err)

	t.Log(addr.String())

}

func TestZreoAddress(t *testing.T) {
	assert := assert.New(t)

	str := ZeroAddress.String()

	maybeZero, err := NewAddrFromString(str)
	assert.NoError(err)

	assert.Equal(ZeroAddress, maybeZero)

}

func TestCommonAddr(t *testing.T) {
	str := "otK5XLQHTym83ygtAk6XanyYSioatrnGTm1jYtddAEVNNKp"
	addr, err := NewAddrFromString(str)
	if err != nil {
		t.Error(err)
	}

	caddr, err := addr.NewCommonAddr()
	if err != nil {
		t.Error(err)
	}

	t.Log(caddr.String())
}

func TestStringToAddress(t *testing.T) {
	str1 := "otK5XLQHTym83ygtAk6XanyYSioatrnGTm1jYtddAEVNNKp"
	addr1, err := StringToAddress(str1)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("addr1:%v\n", addr1)

	str2 := "0xF73d8f5BFb7f03b0AF375b1b5cF6581C367890e8"
	addr2, err := StringToAddress(str2)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("addr2:%v\n", addr2)

	str4 := "5XLQHTym83ygtAk6XanyYSioatrnGTm1jYtddAEVNNKp"
	addr4, err := StringToAddress(str4)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("addr4:%v\n", addr4)
}
