package runtime

import (
	"math"
	"math/big"
	"sync"
)

// Python's float math calls the C library, whose results are correctly
// rounded but for the rarest cases; Go's math package is often an ulp or
// more off (Pow on most inputs). These functions compute in high
// precision with math/big and round once, so results match Python's.
// Special values (zeros, infinities, NaN) follow C99 Annex F, as Go's
// math package does for them.

const wp = 192 // working precision in bits

func newF() *big.Float { return new(big.Float).SetPrec(wp) }

func bf(x float64) *big.Float { return newF().SetFloat64(x) }

func finite(x float64) bool { return !math.IsInf(x, 0) && !math.IsNaN(x) }

var (
	constMu  sync.Mutex
	ln2Cache = map[uint]*big.Float{}
	piCache  = map[uint]*big.Float{}
)

// ln2 returns ln(2) at precision prec: 2*atanh(1/3).
func ln2(prec uint) *big.Float {
	constMu.Lock()
	defer constMu.Unlock()
	if v, ok := ln2Cache[prec]; ok {
		return v
	}
	third := new(big.Float).SetPrec(prec).Quo(new(big.Float).SetPrec(prec).SetInt64(1), new(big.Float).SetPrec(prec).SetInt64(3))
	v := atanhSeries(third, prec)
	v.Mul(v, new(big.Float).SetPrec(prec).SetInt64(2))
	ln2Cache[prec] = v
	return v
}

// pi returns pi at precision prec (Machin: 16 atan(1/5) - 4 atan(1/239)).
func pi(prec uint) *big.Float {
	constMu.Lock()
	defer constMu.Unlock()
	if v, ok := piCache[prec]; ok {
		return v
	}
	one := new(big.Float).SetPrec(prec).SetInt64(1)
	a := atanSeries(new(big.Float).SetPrec(prec).Quo(one, new(big.Float).SetPrec(prec).SetInt64(5)), prec)
	b := atanSeries(new(big.Float).SetPrec(prec).Quo(one, new(big.Float).SetPrec(prec).SetInt64(239)), prec)
	a.Mul(a, new(big.Float).SetPrec(prec).SetInt64(16))
	b.Mul(b, new(big.Float).SetPrec(prec).SetInt64(4))
	v := a.Sub(a, b)
	piCache[prec] = v
	return v
}

// atanhSeries is z + z^3/3 + z^5/5 + ... for small |z|.
func atanhSeries(z *big.Float, prec uint) *big.Float {
	return oddSeries(z, prec, false)
}

// atanSeries is z - z^3/3 + z^5/5 - ... for small |z|.
func atanSeries(z *big.Float, prec uint) *big.Float {
	return oddSeries(z, prec, true)
}

func oddSeries(z *big.Float, prec uint, alternate bool) *big.Float {
	sum := new(big.Float).SetPrec(prec).Set(z)
	z2 := new(big.Float).SetPrec(prec).Mul(z, z)
	term := new(big.Float).SetPrec(prec).Set(z)
	eps := new(big.Float).SetPrec(prec).SetMantExp(big.NewFloat(1), -int(prec)-4)
	for k := int64(3); ; k += 2 {
		term.Mul(term, z2)
		t := new(big.Float).SetPrec(prec).Quo(term, new(big.Float).SetPrec(prec).SetInt64(k))
		if alternate && (k/2)%2 == 1 {
			sum.Sub(sum, t)
		} else {
			sum.Add(sum, t)
		}
		if new(big.Float).Abs(t).Cmp(eps) < 0 {
			return sum
		}
	}
}

// logBig is ln(x) for x > 0.
func logBig(x *big.Float) *big.Float {
	mant := newF()
	e := x.MantExp(mant) // x = mant * 2^e, mant in [0.5, 1)
	if mant.Cmp(big.NewFloat(math.Sqrt2/2)) < 0 {
		mant.Mul(mant, big.NewFloat(2))
		e--
	}
	one := newF().SetInt64(1)
	z := newF().Quo(newF().Sub(mant, one), newF().Add(mant, one))
	r := atanhSeries(z, wp)
	r.Mul(r, newF().SetInt64(2))
	return r.Add(r, newF().Mul(ln2(wp), newF().SetInt64(int64(e))))
}

// expBig is e^x; x must be small enough for the result to be finite in
// big.Float (callers bound it).
func expBig(x *big.Float) *big.Float {
	n, _ := newF().Quo(x, ln2(wp)).Int64()
	r := newF().Sub(x, newF().Mul(ln2(wp), newF().SetInt64(n)))
	// e^r with |r| < 1: halve 16 times, sum the Taylor series, square
	// back.
	const halvings = 16
	r.SetMantExp(r, -halvings)
	sum, term := newF().SetInt64(1), newF().SetInt64(1)
	eps := newF().SetMantExp(big.NewFloat(1), -wp-4)
	for k := int64(1); ; k++ {
		term.Mul(term, r)
		term.Quo(term, newF().SetInt64(k))
		sum.Add(sum, term)
		if newF().Abs(term).Cmp(eps) < 0 {
			break
		}
	}
	for range halvings {
		sum.Mul(sum, sum)
	}
	return sum.SetMantExp(sum, int(n))
}

func round64(x *big.Float) float64 {
	f, _ := x.Float64()
	return f
}

// Log is C's log().
func Log(x float64) float64 {
	if !finite(x) || x <= 0 || x == 1 {
		return math.Log(x)
	}
	return round64(logBig(bf(x)))
}

// Exp is C's exp().
func Exp(x float64) float64 {
	if !finite(x) || x == 0 || x > 710 || x < -746 {
		return math.Exp(x)
	}
	return round64(expBig(bf(x)))
}

// Pow is C's pow() for the real cases (a negative base needs an
// integral exponent).
func Pow(x, y float64) float64 {
	if !finite(x) || !finite(y) || x == 0 || y == 0 || x == 1 || y == 1 {
		return math.Pow(x, y)
	}
	neg := false
	if x < 0 {
		if y != math.Trunc(y) {
			return math.NaN()
		}
		x = -x
		neg = math.Mod(y, 2) != 0
	}
	l := newF().Mul(logBig(bf(x)), bf(y))
	lf, _ := l.Float64()
	var r float64
	switch {
	case lf > 710:
		r = math.Inf(1)
	case lf < -746:
		r = 0
	default:
		r = round64(expBig(l))
	}
	if neg {
		r = -r
	}
	return r
}

// Hypot is C's hypot().
func Hypot(x, y float64) float64 {
	if !finite(x) || !finite(y) {
		return math.Hypot(x, y)
	}
	s := newF().Add(newF().Mul(bf(x), bf(x)), newF().Mul(bf(y), bf(y)))
	return round64(s.Sqrt(s))
}

// reduce returns r = x - k*pi/2 with |r| <= pi/4, and k mod 4.
func reduce(x float64) (*big.Float, int) {
	_, e := math.Frexp(x)
	prec := uint(wp + max(e, 0) + 64)
	halfPi := new(big.Float).SetPrec(prec).SetMantExp(pi(prec), -1)
	xb := new(big.Float).SetPrec(prec).SetFloat64(x)
	q := new(big.Float).SetPrec(prec).Quo(xb, halfPi)
	// Round q to the nearest integer.
	q.Add(q, new(big.Float).SetPrec(prec).SetFloat64(math.Copysign(0.5, x)))
	k, _ := q.Int(nil)
	r := new(big.Float).SetPrec(prec).Sub(xb, new(big.Float).SetPrec(prec).Mul(halfPi, new(big.Float).SetPrec(prec).SetInt(k)))
	m := new(big.Int).Mod(k, big.NewInt(4)).Int64()
	return newF().Set(r), int(m)
}

// sinCosBig returns sin(r) and cos(r) for |r| <= pi/4.
func sinCosBig(r *big.Float) (*big.Float, *big.Float) {
	r2 := newF().Mul(r, r)
	sin, cos := newF().Set(r), newF().SetInt64(1)
	term := newF().Set(r)
	eps := newF().SetMantExp(big.NewFloat(1), -wp-4)
	for k := int64(1); ; k++ {
		term.Mul(term, r2)
		term.Quo(term, newF().SetInt64((2*k)*(2*k+1)))
		term.Neg(term)
		sin.Add(sin, term)
		if newF().Abs(term).Cmp(eps) < 0 {
			break
		}
	}
	term.SetInt64(1)
	for k := int64(1); ; k++ {
		term.Mul(term, r2)
		term.Quo(term, newF().SetInt64((2*k-1)*(2*k)))
		term.Neg(term)
		cos.Add(cos, term)
		if newF().Abs(term).Cmp(eps) < 0 {
			break
		}
	}
	return sin, cos
}

// Sin is C's sin().
func Sin(x float64) float64 {
	if !finite(x) || x == 0 {
		return math.Sin(x)
	}
	r, k := reduce(x)
	s, c := sinCosBig(r)
	switch k {
	case 0:
		return round64(s)
	case 1:
		return round64(c)
	case 2:
		return -round64(s)
	}
	return -round64(c)
}

// Cos is C's cos().
func Cos(x float64) float64 {
	if !finite(x) {
		return math.Cos(x)
	}
	if x == 0 {
		return 1
	}
	r, k := reduce(x)
	s, c := sinCosBig(r)
	switch k {
	case 0:
		return round64(c)
	case 1:
		return -round64(s)
	case 2:
		return -round64(c)
	}
	return round64(s)
}

// atanBig is atan(t) for t >= 0.
func atanBig(t *big.Float) *big.Float {
	one := newF().SetInt64(1)
	invert := t.Cmp(one) > 0
	if invert {
		t = newF().Quo(one, t)
	}
	// Halve the angle until t is small: atan(t) = 2 atan(t / (1 + sqrt(1 + t^2))).
	doublings := 0
	for t.Cmp(big.NewFloat(0.125)) > 0 {
		s := newF().Add(one, newF().Mul(t, t))
		s.Sqrt(s)
		t = newF().Quo(t, s.Add(s, one))
		doublings++
	}
	a := atanSeries(t, wp)
	a.SetMantExp(a, doublings)
	if invert {
		a = newF().Sub(newF().SetMantExp(pi(wp), -1), a)
	}
	return a
}

// Atan2 is C's atan2().
func Atan2(y, x float64) float64 {
	if !finite(x) || !finite(y) || x == 0 || y == 0 {
		return math.Atan2(y, x)
	}
	a := atanBig(newF().Quo(bf(math.Abs(y)), bf(math.Abs(x))))
	if x < 0 {
		a = newF().Sub(pi(wp), a)
	}
	if y < 0 {
		a.Neg(a)
	}
	return round64(a)
}
