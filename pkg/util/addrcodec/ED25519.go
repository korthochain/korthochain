package addrcodec

import "math/big"

func DecodeAddr(b string) []byte {
	answer := big.NewInt(0)
	j := big.NewInt(1)

	scratch := new(big.Int)
	for i := len(b) - 1; i >= 0; i-- {
		tmp := b58[b[i]]
		if tmp == 255 {
			return []byte("")
		}
		scratch.SetInt64(int64(tmp))
		scratch.Mul(j, scratch)
		answer.Add(answer, scratch)
		j.Mul(j, bigRadix)
	}

	tmpval := answer.Bytes()

	var numZeros = 32 - len(tmpval)
	flen := numZeros + len(tmpval)
	val := make([]byte, flen)
	copy(val[numZeros:], tmpval)

	return val
}

func EncodeAddr(data []byte) string {
	x := new(big.Int)
	x.SetBytes(data)

	answer := make([]byte, 0, len(data)*136/100+1)
	for x.Cmp(bigZero) > 0 {
		mod := new(big.Int)
		x.DivMod(x, bigRadix, mod)
		answer = append(answer, alphabet[mod.Int64()])
	}

	// // leading zero bytes
	// for _, i := range data {
	// 	if i != 0 {
	// 		break
	// 	}
	// 	answer = append(answer, alphabetIdx0)
	// }

	// bit alignment
	for len(answer) < (len(data)*136/100 + 1) {
		answer = append(answer, alphabetIdx0)
	}

	// reverse
	alen := len(answer)
	for i := 0; i < alen/2; i++ {
		answer[i], answer[alen-1-i] = answer[alen-1-i], answer[i]
	}

	return string(answer)
}
