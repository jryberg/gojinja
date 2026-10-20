package runtime

import (
	"errors"
	"math"
)

// Python's complex type, as Go's complex128. Templates get one from a
// negative number raised to a fractional power; the arithmetic ports
// CPython's complexobject.c so results match bit for bit.

func toComplex(v any) (complex128, error) {
	if c, ok := v.(complex128); ok {
		return c, nil
	}
	f, err := numFloat(v)
	return complex(f, 0), err
}

// complexBinop applies op when either operand is complex.
func complexBinop(op string, a, b any) (any, error) {
	x, err := toComplex(a)
	if err != nil {
		return nil, err
	}
	y, err := toComplex(b)
	if err != nil {
		return nil, err
	}
	switch op {
	case "+":
		return complex(real(x)+real(y), imag(x)+imag(y)), nil
	case "-":
		return complex(real(x)-real(y), imag(x)-imag(y)), nil
	case "*":
		return complexProd(x, y), nil
	case "/":
		q, ok := complexQuot(x, y)
		if !ok {
			return nil, errors.New("complex division by zero")
		}
		return q, nil
	case "**":
		return complexPow(x, y)
	}
	// complex has no // or %.
	return nil, errNotNumber
}

// complexProd is _Py_c_prod.
func complexProd(a, b complex128) complex128 {
	return complex(real(a)*real(b)-imag(a)*imag(b), real(a)*imag(b)+imag(a)*real(b))
}

// complexQuot is _Py_c_quot (Smith's algorithm); false means division by
// zero.
func complexQuot(a, b complex128) (complex128, bool) {
	ar, ai, br, bi := real(a), imag(a), real(b), imag(b)
	absBr, absBi := math.Abs(br), math.Abs(bi)
	switch {
	case absBr >= absBi:
		if absBr == 0 {
			return 0, false
		}
		ratio := bi / br
		denom := br + bi*ratio
		return complex((ar+ai*ratio)/denom, (ai-ar*ratio)/denom), true
	case absBi >= absBr:
		ratio := br / bi
		denom := br*ratio + bi
		return complex((ar*ratio+ai)/denom, (ai*ratio-ar)/denom), true
	}
	return complex(math.NaN(), math.NaN()), true
}

// complexPow is complex_pow: c_powi for a small integral exponent,
// _Py_c_pow otherwise.
func complexPow(a, b complex128) (any, error) {
	var p complex128
	domain := false
	if imag(b) == 0 && real(b) == math.Floor(real(b)) && math.Abs(real(b)) <= 100 {
		n := int64(real(b))
		if n > 0 {
			p = complexPowu(a, n)
		} else {
			var ok bool
			p, ok = complexQuot(1, complexPowu(a, -n))
			domain = !ok
		}
	} else {
		switch {
		case real(b) == 0 && imag(b) == 0:
			p = 1
		case real(a) == 0 && imag(a) == 0:
			domain = imag(b) != 0 || real(b) < 0
			p = 0
		default:
			vabs := Hypot(real(a), imag(a))
			length := Pow(vabs, real(b))
			at := Atan2(imag(a), real(a))
			phase := at * real(b)
			if imag(b) != 0 {
				length /= Exp(at * imag(b))
				phase += imag(b) * Log(vabs)
			}
			p = complex(length*Cos(phase), length*Sin(phase))
		}
	}
	if domain {
		return nil, errors.New("0.0 to a negative or complex power")
	}
	if math.IsInf(real(p), 0) || math.IsInf(imag(p), 0) {
		return nil, errors.New("complex exponentiation")
	}
	return p, nil
}

// complexPowu is c_powu: x ** n by repeated squaring.
func complexPowu(x complex128, n int64) complex128 {
	r, p := complex128(1), x
	for mask := int64(1); mask > 0 && n >= mask; mask <<= 1 {
		if n&mask != 0 {
			r = complexProd(r, p)
		}
		p = complexProd(p, p)
	}
	return r
}
