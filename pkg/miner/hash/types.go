package hash

import "math/big"

type group struct {
	a *big.Int
	b *big.Int
	c *big.Int
	d *big.Int
}

type pair struct {
	p int64
	q int64
}

const sizeInBits = 2048
