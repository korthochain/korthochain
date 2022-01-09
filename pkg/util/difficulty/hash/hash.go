package hash

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"sort"
)

func Hash64(data []byte) []byte {
	rdata := sha256.Sum256(data)
	return rdata[:]
}

func Hash(data []byte) []byte {
	d := newDiscriminant(data, sizeInBits)
	x := newGroupFromDiscriminant(big.NewInt(2), big.NewInt(1), d)
	a, b := calculate(d, x, 1, sizeInBits)
	if a == nil || b == nil {
		return make([]byte, 32)
	}
	rdata := sha256.Sum256(append(a.Serialize(), b.Serialize()...))
	return rdata[:]
}

var m int
var residues []int
var odd_primes []int
var sieve_info []pair

func init() {
	odd_primes = primeLessThanN(1 << 16)
	m = 8 * 3 * 5 * 7 * 11 * 13
	residues = make([]int, 0, m)
	sieve_info = make([]pair, 0, len(odd_primes))
	for x := 7; x < m; x += 8 {
		if (x%3 != 0) && (x%5 != 0) && (x%7 != 0) && (x%11 != 0) && (x%13 != 0) {
			residues = append(residues, x)
		}
	}

	var odd_primes_above_13 = odd_primes[5:]

	for i := 0; i < len(odd_primes_above_13); i++ {
		prime := int64(odd_primes_above_13[i])
		sieve_info = append(sieve_info, pair{p: int64(prime), q: modExp(int64(m)%prime, prime-2, prime)})
	}
}

func newDiscriminant(seed []byte, length int) *big.Int {
	extra := uint8(length) & 7
	byte_count := ((length + 7) >> 3) + 2
	entropy := entropyFromSeed(seed, byte_count)
	n := new(big.Int)
	n.SetBytes(entropy[:len(entropy)-2])
	n = new(big.Int).Rsh(n, uint(((8 - extra) & 7)))
	n = new(big.Int).SetBit(n, length-1, 1)
	n = new(big.Int).Sub(n, new(big.Int).Mod(n, big.NewInt(int64(m))))
	n = new(big.Int).Add(n, big.NewInt(int64(residues[int(binary.BigEndian.Uint16(entropy[len(entropy)-2:len(entropy)]))%len(residues)])))

	negN := new(big.Int).Neg(n)

	for {
		sieve := make([]bool, (1 << 16))

		for _, v := range sieve_info {
			i := (new(big.Int).Mod(negN, big.NewInt(v.p)).Int64() * v.q) % v.p

			for i < int64(len(sieve)) {
				sieve[i] = true
				i += v.p
			}
		}

		for i, v := range sieve {
			t := new(big.Int).Add(n, big.NewInt(int64(m)*int64(i)))
			if !v && t.ProbablyPrime(1) {
				return new(big.Int).Neg(t)
			}
		}

		bigM := big.NewInt(int64(m))
		n = new(big.Int).Add(n, bigM.Mul(bigM, big.NewInt(int64(1<<16))))
	}
}

func calculate(discriminant *big.Int, x *group, iterations, int_size_bits int) (y, proof *group) {
	L, k, _ := approximateParameters(iterations)

	loopCount := int(math.Ceil(float64(iterations) / float64(k*L)))
	powers_to_calculate := make([]int, loopCount+2)

	for i := 0; i < loopCount+1; i++ {
		powers_to_calculate[i] = i * k * L
	}

	powers_to_calculate[loopCount+1] = iterations

	powers := iterateSquarings(x, powers_to_calculate)

	if powers == nil {
		return nil, nil
	}

	y = powers[iterations]

	identity := identityForDiscriminant(discriminant)

	proof = generateProof(identity, x, y, iterations, k, L, powers)

	return y, proof
}

func approximateParameters(T int) (int, int, int) {
	log_memory := math.Log(10000000) / math.Log(2)
	log_T := math.Log(float64(T)) / math.Log(2)
	L := 1

	if log_T-log_memory > 0 {
		L = int(math.Ceil(math.Pow(2, log_memory-20)))
	}

	intermediate := float64(T) * math.Log(2) / float64(2*L)
	k := int(math.Max(math.Round(math.Log(intermediate)-math.Log(math.Log(intermediate))+0.25), 1))

	w := int(math.Floor(float64(T)/(float64(T)/float64(k)+float64(L)*math.Pow(2, float64(k+1)))) - 2)

	return L, k, w
}

func generateProof(identity, x, y *group, T, k, l int, powers map[int]*group) *group {
	x_s := x.Serialize()
	y_s := y.Serialize()
	B := hashPrime(x_s, y_s)
	proof := evalOptimized(identity, x, B, T, k, l, powers)
	return proof
}

func entropyFromSeed(seed []byte, byte_count int) []byte {
	buffer := bytes.Buffer{}
	bufferSize := 0

	extra := uint16(0)
	for bufferSize <= byte_count {
		extra_bits := make([]byte, 2)
		binary.BigEndian.PutUint16(extra_bits, extra)
		more_entropy := sha256.Sum256(append(seed, extra_bits...)[:])
		buffer.Write(more_entropy[:])
		bufferSize += sha256.Size
		extra += 1
	}

	return buffer.Bytes()[:byte_count]
}

func primeLessThanN(num int) []int {
	sieve := make([]bool, num+1)
	for i := 3; i <= int(math.Floor(math.Sqrt(float64(num)))); i += 2 {
		if sieve[i] == false {
			for j := i * 2; j <= num; j += i {
				sieve[j] = true // cross
			}
		}
	}
	primes := make([]int, 0, num)
	for i := 3; i <= num; i += 2 {
		if sieve[i] == false {
			primes = append(primes, i)
		}
	}
	return primes
}

func hashPrime(x, y []byte) *big.Int {
	var j uint64 = 0
	jBuf := make([]byte, 8)
	z := new(big.Int)
	for {
		binary.BigEndian.PutUint64(jBuf, j)
		s := append([]byte("prime"), jBuf...)
		s = append(s, x...)
		s = append(s, y...)
		checkSum := sha256.Sum256(s[:])
		z.SetBytes(checkSum[:16])
		if z.ProbablyPrime(1) {
			return z
		}
		j++
	}
}

func getBlock(i, k, T int, B *big.Int) *big.Int {
	p1 := big.NewInt(int64(math.Pow(2, float64(k))))
	p2 := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(T-k*(i+1))), B)
	return floorDivision(new(big.Int).Mul(p1, p2), B)
}

func evalOptimized(identity, h *group, B *big.Int, T, k, l int, C map[int]*group) *group {
	var k1 int = k / 2
	k0 := k - k1

	x := cloneGroup(identity)
	for j := l - 1; j > -1; j-- {
		b_limit := int64(math.Pow(2, float64(k)))
		x = x.pow(b_limit)
		if x == nil {
			return nil
		}
		ys := make([]*group, b_limit)
		for b := int64(0); b < b_limit; b++ {
			ys[b] = identity
		}
		for i := 0; i < int(math.Ceil(float64(T)/float64(k*l))); i++ {
			if T-k*(i*l+j+1) < 0 {
				continue
			}
			b := getBlock(i*l+j, k, T, B).Int64()
			ys[b] = ys[b].multiply(C[i*k*l])
			if ys[b] == nil {
				return nil
			}
		}
		for b1 := 0; b1 < int(math.Pow(float64(2), float64(k1))); b1++ {
			z := identity
			for b0 := 0; b0 < int(math.Pow(float64(2), float64((k0)))); b0++ {
				z = z.multiply(ys[int64(b1)*int64(math.Pow(float64(2), float64(k0)))+int64(b0)])
				if z == nil {
					return nil
				}
			}
			c := z.pow(int64(b1) * int64(math.Pow(float64(2), float64(k0))))
			if c == nil {
				return nil
			}
			x = x.multiply(c)
			if x == nil {
				return nil
			}
		}
		for b0 := 0; b0 < int(math.Pow(float64(2), float64(k0))); b0++ {
			z := identity
			for b1 := 0; b1 < int(math.Pow(float64(2), float64(k1))); b1++ {
				z = z.multiply(ys[int64(b1)*int64(math.Pow(float64(2), float64(k0)))+int64(b0)])
				if z == nil {
					return nil
				}
			}
			d := z.pow(int64(b0))
			if d == nil {
				return nil
			}
			x = x.multiply(d)
			if x == nil {
				return nil
			}
		}
	}
	return x
}

func iterateSquarings(x *group, powers_to_calculate []int) map[int]*group {
	powers_calculated := make(map[int]*group)
	previous_power := 0
	currX := cloneGroup(x)
	sort.Ints(powers_to_calculate)
	for _, current_power := range powers_to_calculate {

		for i := 0; i < current_power-previous_power; i++ {
			currX = currX.pow(2)
			if currX == nil {
				return nil
			}
		}
		previous_power = current_power
		powers_calculated[current_power] = currX
	}
	return powers_calculated
}

func modExp(base, exponent, modulus int64) int64 {
	if modulus == 1 {
		return 0
	}
	base = base % modulus
	result := int64(1)
	for i := int64(0); i < exponent; i++ {
		result = (result * base) % modulus
	}
	return result
}
