package difficulty

import (
	"fmt"
	"math/big"
)

var powLimit = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(1))
var Cycle = 10

// func NextMinerDifficulty(baseDifficulty *big.Int, minerPledge, basePledge uint64) (*big.Int, error) {
// 	target, baseTargetCopy := big.NewInt(0), big.NewInt(0)

// 	minerTarget := big.NewInt(0)
// 	target.Add(target, baseDifficulty)
// 	baseTargetCopy.Add(baseTargetCopy, baseDifficulty)

// 	if minerPledge > basePledge {
// 		minerPledge = basePledge
// 	}

// 	a := big.NewInt(0).SetUint64((minerPledge) * 900)
// 	a = a.Div(a, big.NewInt(0).SetUint64(basePledge))

// 	minerTarget.Add(baseDifficulty, target.Mul(target, a).Div(target, big.NewInt(100)))

// 	if baseTargetCopy.Cmp(minerTarget) > 0 {
// 		return nil, fmt.Errorf("miners too difficult,old difficulty:%s new difficulty:%s miner pledge:%d,basfe pledeg:%d",
// 			baseTargetCopy, minerTarget, minerPledge, basePledge)
// 	} else if baseTargetCopy.Mul(baseTargetCopy, big.NewInt(10)).Cmp(minerTarget) < 0 {
// 		return nil, fmt.Errorf("miners too little difficulty,old difficulty:%s new difficulty:%s miner pledge:%d,basfe pledeg:%d",
// 			baseTargetCopy, minerTarget, minerPledge, basePledge)
// 	}

// 	return minerTarget, nil
// }

func NextMinerDifficulty(baseDifficulty *big.Int, minerPledge, basePledge uint64) (*big.Int, error) {
	minerTarget := big.NewInt(0)
	baseTargetCopy := new(big.Int).Set(baseDifficulty)

	if minerPledge > basePledge {
		minerPledge = basePledge
	}

	a := new(big.Float).SetUint64(minerPledge)
	a = new(big.Float).Quo(a, new(big.Float).SetUint64(basePledge))

	a.Mul(a, big.NewFloat(9)).Mul(a, big.NewFloat(0).SetInt(baseDifficulty))
	a.Add(a, big.NewFloat(0).SetInt(baseDifficulty)).Int(minerTarget)

	if baseTargetCopy.Cmp(minerTarget) > 0 {
		return nil, fmt.Errorf("miners too difficult,old difficulty:%s new difficulty:%s miner pledge:%d,basfe pledeg:%d",
			baseTargetCopy, minerTarget, minerPledge, basePledge)
	} else if baseTargetCopy.Mul(baseTargetCopy, big.NewInt(10)).Cmp(minerTarget) < 0 {
		return nil, fmt.Errorf("miners too little difficulty,old difficulty:%s new difficulty:%s miner pledge:%d,basfe pledeg:%d",
			baseTargetCopy, minerTarget, minerPledge, basePledge)
	}

	return minerTarget, nil
}

func CalcNextGlobalRequiredDifficulty(t0, t1 int64, globalBits uint32) uint32 {

	actualTimespan := t1 - t0
	adjustedTimespan := actualTimespan

	oldTarget := CompactToBig(globalBits)
	newTarget := new(big.Int).Mul(oldTarget, big.NewInt(adjustedTimespan))
	targetTimeSpan := int64(Cycle)
	newTarget.Div(newTarget, big.NewInt(targetTimeSpan*10))

	if newTarget.Cmp(powLimit) > 0 {
		newTarget.Set(powLimit)
	}

	newTargetBits := BigToCompact(newTarget)
	return newTargetBits
}

func CompactToBig(compact uint32) *big.Int {
	mantissa := compact & 0x007fffff
	//  0010 0000 0111 1111 1111 1111 1111 1111
	//  0000 0000 0111 1111 1111 1111 1111 1111
	isNegative := compact&0x00800000 != 0
	exponent := uint(compact >> 24)
	var bn *big.Int
	if exponent <= 3 {
		mantissa >>= 8 * (3 - exponent)
		bn = big.NewInt(int64(mantissa))
	} else {
		bn = big.NewInt(int64(mantissa))
		bn.Lsh(bn, 8*(exponent-3))
	}
	if isNegative {
		bn = bn.Neg(bn)
	}
	return bn
}

func BigToCompact(n *big.Int) uint32 {
	if n.Sign() == 0 {
		return 0
	}
	var mantissa uint32
	exponent := uint(len(n.Bytes()))
	if exponent <= 3 {
		mantissa = uint32(n.Bits()[0])
		mantissa <<= 8 * (3 - exponent)
	} else {
		tn := new(big.Int).Set(n)
		mantissa = uint32(tn.Rsh(tn, 8*(exponent-3)).Bits()[0])
	}
	if mantissa&0x00800000 != 0 {
		mantissa >>= 8
		exponent++
	}
	compact := uint32(exponent<<24) | mantissa
	if n.Sign() < 0 {
		compact |= 0x00800000
	}
	return compact
}

func HashToBig(buf []byte) *big.Int {
	blen := len(buf)
	for i := 0; i < blen/2; i++ {
		buf[i], buf[blen-1-i] = buf[blen-1-i], buf[i]
	}
	return new(big.Int).SetBytes(buf[:])
}
