package runtime

import (
	"errors"
	"fmt"
	"math"
	"math/big"
)

// Python's arithmetic on numbers: ints (any Go integer or *big.Int, bool
// counting as 0 or 1) and floats. Ints are exact and unbounded (up to
// MaxIntBits); an int meets a float as a float.

// IsNumber reports whether v is a Python number (bool included).
func IsNumber(v any) bool {
	switch v.(type) {
	case bool, float64, float32, complex128:
		return true
	}
	return IsInt(v)
}

func numInt(v any) (*big.Int, bool) { return intOrBool(v) }

// smallInt returns v as an int64 when it is an int or bool that fits.
func smallInt(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case *big.Int:
		return 0, false
	}
	if b, ok := BigInt(v); ok && b.IsInt64() {
		return b.Int64(), true
	}
	return 0, false
}

// numFloat converts a number to a float, an int as float(int) does.
func numFloat(v any) (float64, error) {
	if f, ok := floatOf(v); ok {
		return f, nil
	}
	if b, ok := v.(bool); ok {
		if b {
			return 1, nil
		}
		return 0, nil
	}
	return IntToFloat(v)
}

// NumBinop applies the arithmetic operator op (+ - * / // % **) to two
// numbers with Python's semantics and errors. ok is false when a or b is
// not a number.
func NumBinop(op string, a, b any) (result any, err error) {
	if !IsNumber(a) || !IsNumber(b) {
		return nil, errNotNumber
	}
	_, aComplex := a.(complex128)
	_, bComplex := b.(complex128)
	if aComplex || bComplex {
		return complexBinop(op, a, b)
	}
	_, aFloat := floatOf(a)
	_, bFloat := floatOf(b)
	if !aFloat && !bFloat {
		return intBinop(op, a, b)
	}
	x, err := numFloat(a)
	if err != nil {
		return nil, err
	}
	y, err := numFloat(b)
	if err != nil {
		return nil, err
	}
	return floatBinop(op, x, y)
}

var errNotNumber = errors.New("not a number")

// IsNotNumber reports whether err is NumBinop's error for an operand
// that is not a number.
func IsNotNumber(err error) bool { return err == errNotNumber }

func intBinop(op string, a, b any) (any, error) {
	x, xSmall := smallInt(a)
	y, ySmall := smallInt(b)
	if xSmall && ySmall {
		if r, ok := smallBinop(op, x, y); ok {
			return r, nil
		}
	}
	bx, _ := numInt(a)
	by, _ := numInt(b)
	switch op {
	case "+":
		return NormInt(new(big.Int).Add(bx, by))
	case "-":
		return NormInt(new(big.Int).Sub(bx, by))
	case "*":
		if bx.BitLen()+by.BitLen() > MaxIntBits+1 {
			return nil, ErrIntTooLarge
		}
		return NormInt(new(big.Int).Mul(bx, by))
	case "/":
		if by.Sign() == 0 {
			return nil, errors.New("division by zero")
		}
		f, _ := new(big.Rat).SetFrac(bx, by).Float64()
		if math.IsInf(f, 0) {
			return nil, errors.New("integer division result too large for a float")
		}
		return f, nil
	case "//", "%":
		if by.Sign() == 0 {
			if op == "//" {
				return nil, errors.New("integer division or modulo by zero")
			}
			return nil, errors.New("integer modulo by zero")
		}
		q, r := new(big.Int).QuoRem(bx, by, new(big.Int))
		// Python floors the quotient; the remainder takes the divisor's
		// sign.
		if r.Sign() != 0 && r.Sign() != by.Sign() {
			q.Sub(q, big.NewInt(1))
			r.Add(r, by)
		}
		if op == "//" {
			return NormInt(q)
		}
		return NormInt(r)
	case "**":
		return intPow(bx, by)
	}
	return nil, fmt.Errorf("unknown binary operator %q", op)
}

// smallBinop computes op on int64s, reporting false on overflow or for
// the operations the big path handles (/, **).
func smallBinop(op string, x, y int64) (int64, bool) {
	switch op {
	case "+":
		r := x + y
		if (x > 0 && y > 0 && r < 0) || (x < 0 && y < 0 && r >= 0) {
			return 0, false
		}
		return r, true
	case "-":
		r := x - y
		if (x >= 0 && y < 0 && r < 0) || (x < 0 && y > 0 && r >= 0) {
			return 0, false
		}
		return r, true
	case "*":
		if x == 0 || y == 0 {
			return 0, true
		}
		r := x * y
		if r/y != x || (x == -1 && y == math.MinInt64) || (y == -1 && x == math.MinInt64) {
			return 0, false
		}
		return r, true
	case "//":
		if y == 0 || (x == math.MinInt64 && y == -1) {
			return 0, false
		}
		q := x / y
		if x%y != 0 && (x < 0) != (y < 0) {
			q--
		}
		return q, true
	case "%":
		if y == 0 {
			return 0, false
		}
		r := x % y
		if r != 0 && (r < 0) != (y < 0) {
			r += y
		}
		return r, true
	}
	return 0, false
}

// intPow is int ** int: exact for a non-negative exponent, a float for a
// negative one.
func intPow(x, y *big.Int) (any, error) {
	if y.Sign() < 0 {
		fx, err := IntToFloat(x)
		if err != nil {
			return nil, err
		}
		fy, err := IntToFloat(y)
		if err != nil {
			return nil, err
		}
		return floatBinop("**", fx, fy)
	}
	// 0, 1 and -1 stay small whatever the exponent.
	if x.CmpAbs(big.NewInt(1)) <= 0 {
		if x.Sign() < 0 && y.Bit(0) == 0 {
			return int64(1), nil
		}
		if x.Sign() == 0 && y.Sign() == 0 {
			return int64(1), nil
		}
		return NormInt(x)
	}
	if !y.IsInt64() || y.Int64() > MaxIntBits || int64(x.BitLen()-1)*y.Int64() > MaxIntBits {
		return nil, ErrIntTooLarge
	}
	return NormInt(new(big.Int).Exp(x, y, nil))
}

func floatBinop(op string, x, y float64) (any, error) {
	switch op {
	case "+":
		return x + y, nil
	case "-":
		return x - y, nil
	case "*":
		return x * y, nil
	case "/":
		if y == 0 {
			return nil, errors.New("float division by zero")
		}
		return x / y, nil
	case "//":
		if y == 0 {
			return nil, errors.New("float floor division by zero")
		}
		div, _ := floatDivmod(x, y)
		return div, nil
	case "%":
		if y == 0 {
			return nil, errors.New("float modulo")
		}
		_, mod := floatDivmod(x, y)
		return mod, nil
	case "**":
		return floatPow(x, y)
	}
	return nil, fmt.Errorf("unknown binary operator %q", op)
}

// floatDivmod is CPython's float_divmod: the floored quotient and the
// remainder with the divisor's sign.
func floatDivmod(vx, wx float64) (float64, float64) {
	mod := math.Mod(vx, wx)
	div := (vx - mod) / wx
	if mod != 0 {
		if (wx < 0) != (mod < 0) {
			mod += wx
			div -= 1
		}
	} else {
		mod = math.Copysign(0, wx)
	}
	var floordiv float64
	if div != 0 {
		floordiv = math.Floor(div)
		if div-floordiv > 0.5 {
			floordiv++
		}
	} else {
		floordiv = math.Copysign(0, vx/wx)
	}
	return floordiv, mod
}

// floatPow is CPython's float_pow for real results: 0 to a negative
// power is a ZeroDivisionError and a finite overflow an OverflowError.
func floatPow(x, y float64) (any, error) {
	if x == 0 && y < 0 && !math.IsInf(y, 0) {
		return nil, errors.New("0.0 cannot be raised to a negative power")
	}
	if x < 0 && y != math.Trunc(y) && !math.IsInf(y, 0) && !math.IsNaN(y) {
		return complexPow(complex(x, 0), complex(y, 0))
	}
	r := Pow(x, y)
	if math.IsInf(r, 0) && !math.IsInf(x, 0) && !math.IsInf(y, 0) {
		return nil, errors.New("(34, 'Numerical result out of range')")
	}
	return r, nil
}

// NegateNumber is unary minus on a number.
func NegateNumber(v any) (any, error) {
	switch x := v.(type) {
	case float64:
		return -x, nil
	case float32:
		return -float64(x), nil
	case complex128:
		return complex(-real(x), -imag(x)), nil
	case bool:
		if x {
			return int64(-1), nil
		}
		return int64(0), nil
	}
	if i, ok := smallInt(v); ok && i != math.MinInt64 {
		return -i, nil
	}
	b, ok := BigInt(v)
	if !ok {
		return nil, errNotNumber
	}
	return NormInt(new(big.Int).Neg(b))
}

// AbsNumber is Python's abs() of a number: exact for an int (bool
// included), the magnitude for a complex.
func AbsNumber(v any) (any, error) {
	switch x := v.(type) {
	case float64:
		return math.Abs(x), nil
	case float32:
		return math.Abs(float64(x)), nil
	case complex128:
		return Hypot(real(x), imag(x)), nil
	case bool:
		if x {
			return int64(1), nil
		}
		return int64(0), nil
	}
	b, ok := BigInt(v)
	if !ok {
		return nil, errNotNumber
	}
	return NormInt(new(big.Int).Abs(b))
}
