package hash

import "math/big"

func newGroup(a, b, c *big.Int) *group {
	return &group{a: a, b: b, c: c}
}

func cloneGroup(g *group) *group {
	return newGroup(g.a, g.b, g.c)
}

func newGroupFromDiscriminant(a, b, d *big.Int) *group {
	z := new(big.Int).Sub(new(big.Int).Mul(b, b), d)
	c := floorDivision(z, new(big.Int).Mul(a, big.NewInt(4)))
	return newGroup(a, b, c)
}

func identityForDiscriminant(d *big.Int) *group {
	return newGroupFromDiscriminant(big.NewInt(1), big.NewInt(1), d)
}

func (g *group) identity() *group {
	return newGroupFromDiscriminant(big.NewInt(1), big.NewInt(1), g.discriminant())
}

func (g *group) Serialize() []byte {
	return append(append(g.a.Bytes(), g.b.Bytes()...), g.c.Bytes()...)
}

func (g *group) pow(n int64) *group {
	x := cloneGroup(g)
	items_prod := g.identity()

	for n > 0 {
		if n&1 == 1 {
			items_prod = items_prod.multiply(x)
			if items_prod == nil {
				return nil
			}
		}
		x = x.square()
		if x == nil {
			return nil
		}
		n >>= 1
	}
	return items_prod
}

func (g *group) reduced() *group {
	g = g.normalized()
	a := new(big.Int).Set(g.a)
	b := new(big.Int).Set(g.b)
	c := new(big.Int).Set(g.c)
	for (a.Cmp(c) == 1) || ((a.Cmp(c) == 0) && (b.Sign() == -1)) {
		s := new(big.Int).Add(c, b)
		s = floorDivision(s, new(big.Int).Add(c, c))
		oldA := new(big.Int).Set(a)
		oldB := new(big.Int).Set(b)
		a = new(big.Int).Set(c)
		b.Neg(b)
		x := new(big.Int).Mul(big.NewInt(2), s)
		x.Mul(x, c)
		b.Add(b, x)
		c.Mul(c, s)
		c.Mul(c, s)
		oldB.Mul(oldB, s)
		c.Sub(c, oldB)
		c.Add(c, oldA)
	}
	return newGroup(a, b, c).normalized()
}

func (g *group) normalized() *group {
	a := new(big.Int).Set(g.a)
	b := new(big.Int).Set(g.b)
	c := new(big.Int).Set(g.c)
	if (b.Cmp(new(big.Int).Neg(a)) == 1) && (b.Cmp(a) < 1) {
		return g
	}
	r := new(big.Int).Sub(a, b)
	r = floorDivision(r, new(big.Int).Mul(a, big.NewInt(2)))
	t := new(big.Int).Mul(big.NewInt(2), r)
	t.Mul(t, a)
	oldB := new(big.Int).Set(b)
	b.Add(b, t)
	x := new(big.Int).Mul(a, r)
	x.Mul(x, r)
	y := new(big.Int).Mul(oldB, r)
	c.Add(c, x)
	c.Add(c, y)
	return newGroup(a, b, c)
}

func (g *group) discriminant() *big.Int {
	if g.d == nil {
		d := new(big.Int).Set(g.b)
		d.Mul(d, d)
		a := new(big.Int).Set(g.a)
		a.Mul(a, g.c)
		a.Mul(a, big.NewInt(4))
		d.Sub(d, a)
		g.d = d
	}
	return g.d
}

func (g0 *group) multiply(g1 *group) *group {
	x := g0.reduced()
	y := g1.reduced()
	g := new(big.Int).Add(x.b, y.b)
	g = floorDivision(g, big.NewInt(2))
	h := new(big.Int).Sub(y.b, x.b)
	h = floorDivision(h, big.NewInt(2))
	w1 := allInputValueGCD(y.a, g)
	w := allInputValueGCD(x.a, w1)
	j := new(big.Int).Set(w)
	r := big.NewInt(0)
	s := floorDivision(x.a, w)
	t := floorDivision(y.a, w)
	u := floorDivision(g, w)
	b := new(big.Int).Mul(h, u)
	sc := new(big.Int).Mul(s, x.c)
	b.Add(b, sc)
	k_temp, constant_factor, solvable := solveMod(new(big.Int).Mul(t, u), b, new(big.Int).Mul(s, t))
	if !solvable {
		return nil
	}
	n, _, solvable := solveMod(new(big.Int).Mul(t, constant_factor), new(big.Int).Sub(h, new(big.Int).Mul(t, k_temp)), s)
	if !solvable {
		return nil
	}
	k := new(big.Int).Add(k_temp, new(big.Int).Mul(constant_factor, n))
	l := floorDivision(new(big.Int).Sub(new(big.Int).Mul(t, k), h), s)
	tuk := new(big.Int).Mul(t, u)
	tuk.Mul(tuk, k)
	hu := new(big.Int).Mul(h, u)
	tuk.Sub(tuk, hu)
	tuk.Sub(tuk, sc)
	st := new(big.Int).Mul(s, t)
	m := floorDivision(tuk, st)
	ru := new(big.Int).Mul(r, u)
	a3 := st.Sub(st, ru)
	ju := new(big.Int).Mul(j, u)
	mr := new(big.Int).Mul(m, r)
	ju = ju.Add(ju, mr)
	kt := new(big.Int).Mul(k, t)
	ls := new(big.Int).Mul(l, s)
	kt = kt.Add(kt, ls)
	b3 := ju.Sub(ju, kt)
	kl := new(big.Int).Mul(k, l)
	jm := new(big.Int).Mul(j, m)
	c3 := kl.Sub(kl, jm)
	return newGroup(a3, b3, c3).reduced()
}

func floorDivision(x, y *big.Int) *big.Int {
	var r big.Int
	q, _ := new(big.Int).QuoRem(x, y, &r)
	if (r.Sign() == 1 && y.Sign() == -1) || (r.Sign() == -1 && y.Sign() == 1) {
		q.Sub(q, big.NewInt(1))
	}
	return q
}

func encoding(buf []byte, bytes_size int) []byte {
	var carry uint8 = 1

	for i := len(buf) - 1; i >= len(buf)-bytes_size; i-- {
		thisdigit := uint8(buf[i])
		thisdigit = thisdigit ^ 0xff
		if thisdigit == 0xff {
			if carry == 1 {
				thisdigit = 0
				carry = 1
			} else {
				carry = 0
			}
		} else {
			thisdigit = thisdigit + carry
			carry = 0
		}
		buf[i] = thisdigit
	}
	for i := len(buf) - bytes_size - 1; i >= 0; i-- {
		buf[i] = 0xff
	}
	return buf
}

func allInputValueGCD(a, b *big.Int) (r *big.Int) {
	if a.Sign() == 0 {
		return new(big.Int).Abs(b)
	}
	if b.Sign() == 0 {
		return new(big.Int).Abs(a)
	}
	return new(big.Int).GCD(nil, nil, new(big.Int).Abs(a), new(big.Int).Abs(b))
}

func solveMod(a, b, m *big.Int) (s, t *big.Int, solvable bool) {
	g, d, _ := extendedGCD(a, m)
	r := big.NewInt(1)
	bb := new(big.Int).Set(b)
	q, r := bb.DivMod(b, g, r)
	if r.Cmp(big.NewInt(0)) != 0 {
		return nil, nil, false
	}
	q.Mul(q, d)
	s = q.Mod(q, m)
	t = floorDivision(m, g)
	return s, t, true
}

func extendedGCD(a, b *big.Int) (r, s, t *big.Int) {
	r0 := new(big.Int).Set(a)
	r1 := new(big.Int).Set(b)
	s0 := big.NewInt(1)
	s1 := big.NewInt(0)
	t0 := big.NewInt(0)
	t1 := big.NewInt(1)
	if r0.Cmp(r1) == 1 {
		oldR0 := new(big.Int).Set(r0)
		r0 = r1
		r1 = oldR0
		oldS0 := new(big.Int).Set(s0)
		s0 = t0
		oldS1 := new(big.Int).Set(s1)
		s1 = t1
		t0 = oldS0
		t1 = oldS1
	}
	for r1.Sign() == 1 {
		r := big.NewInt(1)
		bb := new(big.Int).Set(b)
		q, r := bb.DivMod(r0, r1, r)
		r0 = r1
		r1 = r
		oldS0 := new(big.Int).Set(s0)
		s0 = s1
		s1 = new(big.Int).Sub(oldS0, new(big.Int).Mul(q, s1))
		oldT0 := new(big.Int).Set(t0)
		t0 = t1
		t1 = new(big.Int).Sub(oldT0, new(big.Int).Mul(q, t1))
	}
	return r0, s0, t0
}

func (g *group) square() *group {
	u, _, solvable := solveMod(g.b, g.c, g.a)
	if !solvable {
		return nil
	}
	A := new(big.Int).Mul(g.a, g.a)
	au := new(big.Int).Mul(g.a, u)
	B := new(big.Int).Sub(g.b, new(big.Int).Mul(au, big.NewInt(2)))
	C := new(big.Int).Mul(u, u)
	m := new(big.Int).Mul(g.b, u)
	m = new(big.Int).Sub(m, g.c)
	m = floorDivision(m, g.a)
	C = new(big.Int).Sub(C, m)
	return newGroup(A, B, C).reduced()
}
