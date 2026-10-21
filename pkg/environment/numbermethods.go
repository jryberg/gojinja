package environment

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/jryberg/gojinja/internal/pyunicode"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// The attributes and methods of Python's int (and bool), float and
// complex, with CPython's argument checking and messages.

// numberAttr returns attr of a number, and whether the number has it.
func numberAttr(v any, attr string) (any, bool) {
	switch x := v.(type) {
	case float64:
		return floatAttr(x, attr)
	case float32:
		return floatAttr(float64(x), attr)
	case complex128:
		return complexAttr(x, attr)
	case bool:
		n := int64(0)
		if x {
			n = 1
		}
		return intAttr(big.NewInt(n), attr)
	}
	if n, ok := runtime.BigInt(v); ok {
		return intAttr(n, attr)
	}
	return nil, false
}

func normInt(n *big.Int) any {
	v, err := runtime.NormInt(n)
	if err != nil {
		return n
	}
	return v
}

// noArgsMethod wraps a METH_NOARGS method.
func noArgsMethod(qualname string, f func() (any, error)) any {
	return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
		if err := noArgsCheck(qualname, args, kwargs); err != nil {
			return nil, err
		}
		return f()
	})
}

func intAttr(n *big.Int, attr string) (any, bool) {
	switch attr {
	case "real", "numerator":
		return normInt(n), true
	case "imag":
		return int64(0), true
	case "denominator":
		return int64(1), true
	case "bit_length":
		return noArgsMethod("int.bit_length", func() (any, error) { return int64(new(big.Int).Abs(n).BitLen()), nil }), true
	case "bit_count":
		return noArgsMethod("int.bit_count", func() (any, error) {
			count := 0
			for _, w := range new(big.Int).Abs(n).Bits() {
				for ; w != 0; w &= w - 1 {
					count++
				}
			}
			return int64(count), nil
		}), true
	case "conjugate":
		return noArgsMethod("int.conjugate", func() (any, error) { return normInt(n), nil }), true
	case "as_integer_ratio":
		return noArgsMethod("int.as_integer_ratio", func() (any, error) { return runtime.Tuple{normInt(n), int64(1)}, nil }), true
	case "is_integer":
		return noArgsMethod("int.is_integer", func() (any, error) { return true, nil }), true
	case "to_bytes":
		return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) { return intToBytes(n, args, kwargs) }), true
	case "from_bytes":
		return KwargsCallable(intFromBytes), true
	}
	return nil, false
}

// bytesArgs binds the (length-or-bytes, byteorder='big', *, signed=False)
// parameters of to_bytes and from_bytes.
func bytesArgs(name, first string, firstDefault any, args []any, kwargs map[string]any) (any, string, bool, error) {
	if len(args) > 2 {
		return nil, "", false, methodError("%s() takes at most 2 positional arguments (%d given)", name, len(args))
	}
	params := []string{first, "byteorder"}
	vals := []any{firstDefault, "big"}
	copy(vals, args)
	signed := false
	for k, v := range kwargs {
		switch k {
		case "signed":
			signed = eval.Truthy(v)
			continue
		case params[0], params[1]:
			i := 0
			if k == params[1] {
				i = 1
			}
			if i < len(args) {
				return nil, "", false, methodError("argument for %s() given by name ('%s') and position (%d)", name, k, i+1)
			}
			vals[i] = v
			continue
		}
		return nil, "", false, methodError("%s is an invalid keyword argument for %s()", escape.Repr(k), name)
	}
	if vals[0] == nil {
		return nil, "", false, methodError("%s() missing required argument '%s' (pos 1)", name, first)
	}
	order, ok := eqString(vals[1])
	if !ok {
		return nil, "", false, methodError("%s() argument 'byteorder' must be str, not %s", name, runtime.PyTypeName(vals[1]))
	}
	if order != "big" && order != "little" {
		return nil, "", false, methodError("byteorder must be either 'little' or 'big'")
	}
	return vals[0], order, signed, nil
}

// intToBytes is int.to_bytes(length=1, byteorder='big', *, signed=False).
func intToBytes(n *big.Int, args []any, kwargs map[string]any) (any, error) {
	lv, order, signed, err := bytesArgs("to_bytes", "length", int64(1), args, kwargs)
	if err != nil {
		return nil, err
	}
	length, err := indexArg(lv)
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, methodError("length argument must be non-negative")
	}
	if n.Sign() < 0 && !signed {
		return nil, methodError("can't convert negative int to unsigned")
	}
	// Two's complement in length bytes.
	limit := new(big.Int).Lsh(big.NewInt(1), uint(8*length))
	v := new(big.Int).Set(n)
	if signed {
		half := new(big.Int).Rsh(limit, 1)
		if v.Cmp(half) >= 0 || v.Cmp(new(big.Int).Neg(half)) < 0 {
			return nil, methodError("int too big to convert")
		}
		if v.Sign() < 0 {
			v.Add(v, limit)
		}
	} else if v.Cmp(limit) >= 0 {
		return nil, methodError("int too big to convert")
	}
	out := make(runtime.Bytes, length)
	v.FillBytes(out)
	if order == "little" {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out, nil
}

// intFromBytes is int.from_bytes(bytes, byteorder='big', *, signed=False).
func intFromBytes(args []any, kwargs map[string]any) (any, error) {
	src, order, signed, err := bytesArgs("from_bytes", "bytes", nil, args, kwargs)
	if err != nil {
		return nil, err
	}
	var data []byte
	switch x := src.(type) {
	case runtime.Bytes:
		data = append([]byte(nil), x...)
	case string, escape.Markup:
		return nil, methodError("cannot convert '%s' object to bytes", runtime.PyTypeName(src))
	default:
		items, err := eval.ToIterable(src)
		if err != nil {
			return nil, methodError("cannot convert '%s' object to bytes", runtime.PyTypeName(src))
		}
		for _, it := range items {
			b, err := indexArg(it)
			if err != nil {
				return nil, err
			}
			if b < 0 || b > 255 {
				return nil, methodError("bytes must be in range(0, 256)")
			}
			data = append(data, byte(b))
		}
	}
	if order == "little" {
		for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
			data[i], data[j] = data[j], data[i]
		}
	}
	v := new(big.Int).SetBytes(data)
	if signed && len(data) > 0 && data[0]&0x80 != 0 {
		v.Sub(v, new(big.Int).Lsh(big.NewInt(1), uint(8*len(data))))
	}
	return runtime.NormInt(v)
}

func floatAttr(f float64, attr string) (any, bool) {
	switch attr {
	case "real":
		return f, true
	case "imag":
		return 0.0, true
	case "conjugate":
		return noArgsMethod("float.conjugate", func() (any, error) { return f, nil }), true
	case "is_integer":
		return noArgsMethod("float.is_integer", func() (any, error) {
			return !math.IsInf(f, 0) && !math.IsNaN(f) && f == math.Trunc(f), nil
		}), true
	case "as_integer_ratio":
		return noArgsMethod("float.as_integer_ratio", func() (any, error) {
			switch {
			case math.IsNaN(f):
				return nil, methodError("cannot convert NaN to integer ratio")
			case math.IsInf(f, 0):
				return nil, methodError("cannot convert Infinity to integer ratio")
			}
			r := new(big.Rat).SetFloat64(f)
			return runtime.Tuple{normInt(r.Num()), normInt(r.Denom())}, nil
		}), true
	case "hex":
		return noArgsMethod("float.hex", func() (any, error) { return floatHex(f), nil }), true
	case "fromhex":
		return KwargsCallable(func(args []any, kwargs map[string]any) (any, error) {
			if err := oneArg("float.fromhex", args, kwargs); err != nil {
				return nil, err
			}
			s, ok := eqString(args[0])
			if !ok {
				return nil, methodError("bad argument type for built-in operation")
			}
			return floatFromHex(s)
		}), true
	}
	return nil, false
}

// floatHex is float.hex(): 0x1.<13 hex digits>p<exp>, subnormals as
// 0x0.<digits>p-1022.
func floatHex(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	sign := ""
	if math.Signbit(f) {
		sign = "-"
	}
	if f == 0 {
		return sign + "0x0.0p+0"
	}
	bits := math.Float64bits(f)
	exp := int(bits>>52&0x7ff) - 1023
	mant := bits & (1<<52 - 1)
	lead := 1
	if exp == -1023 {
		lead, exp = 0, -1022
	}
	return fmt.Sprintf("%s0x%d.%013xp%+d", sign, lead, mant, exp)
}

// floatFromHex is float.fromhex(s): correctly rounded, with Python's
// errors.
func floatFromHex(s string) (any, error) {
	invalid := methodError("invalid hexadecimal floating-point string")
	// float_fromhex skips C's isspace, the ASCII whitespace only.
	t := strings.Trim(s, " \t\n\v\f\r")
	neg := false
	if t != "" && (t[0] == '+' || t[0] == '-') {
		neg, t = t[0] == '-', t[1:]
	}
	sign := 1.0
	if neg {
		sign = -1
	}
	switch pyunicode.LowerASCII(t) {
	case "inf", "infinity":
		return math.Copysign(math.Inf(1), sign), nil
	case "nan":
		return math.NaN(), nil
	}
	if len(t) >= 2 && t[0] == '0' && (t[1] == 'x' || t[1] == 'X') {
		t = t[2:]
	}
	mantStr, expStr, hasExp := strings.Cut(pyunicode.LowerASCII(t), "p")
	intPart, frac, _ := strings.Cut(mantStr, ".")
	digits := intPart + frac
	if digits == "" || strings.Contains(frac, ".") {
		return nil, invalid
	}
	m, ok := new(big.Int).SetString(digits, 16)
	if !ok {
		return nil, invalid
	}
	exp := 0
	if hasExp {
		e, err := strconv.Atoi(expStr)
		if err != nil || strings.HasPrefix(expStr, "+-") {
			return nil, invalid
		}
		exp = e
	}
	exp -= 4 * len(frac)
	if m.Sign() == 0 {
		return math.Copysign(0, sign), nil
	}
	// Values far outside the float range need no exact arithmetic.
	if top := m.BitLen() + exp; top > 1025 {
		return nil, methodError("hexadecimal value too large to represent as a float")
	} else if top < -1100 {
		return math.Copysign(0, sign), nil
	}
	r := new(big.Rat).SetInt(m)
	if exp >= 0 {
		r.Mul(r, new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(exp))))
	} else {
		r.Quo(r, new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(-exp))))
	}
	f, _ := r.Float64()
	if math.IsInf(f, 0) {
		return nil, methodError("hexadecimal value too large to represent as a float")
	}
	return math.Copysign(f, sign), nil
}

func complexAttr(c complex128, attr string) (any, bool) {
	switch attr {
	case "real":
		return real(c), true
	case "imag":
		return imag(c), true
	case "conjugate":
		return noArgsMethod("complex.conjugate", func() (any, error) { return complex(real(c), -imag(c)), nil }), true
	}
	return nil, false
}
