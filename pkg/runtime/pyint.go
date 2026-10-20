package runtime

import (
	"errors"
	"math"
	"math/big"
	"reflect"
)

// Python's int is unbounded. Template ints are int64 when they fit and
// *big.Int beyond; every operation returns the int64 form whenever the
// value fits, so a *big.Int always lies outside int64's range.

// MaxIntBits bounds an int's size. Python's ints only stop at memory,
// so `9 ** 9 ** 9` would tie up a render for minutes; past this size an
// operation fails instead. No such int can even be printed in Python,
// which converts at most 4300 decimal digits.
const MaxIntBits = 1 << 22

// ErrIntTooLarge is the error for an int result over MaxIntBits.
var ErrIntTooLarge = errors.New("int too large: results over 4194304 bits are not supported")

// ErrIntToFloat is Python's OverflowError for an int no float can hold.
var ErrIntToFloat = errors.New("int too large to convert to float")

// BigInt returns v as a *big.Int when v is a Python int: any Go integer
// or a *big.Int (bool, which Python also treats as an int, is not).
func BigInt(v any) (*big.Int, bool) {
	switch x := v.(type) {
	case int64:
		return big.NewInt(x), true
	case int:
		return big.NewInt(int64(x)), true
	case *big.Int:
		return x, true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return big.NewInt(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return new(big.Int).SetUint64(rv.Uint()), true
	}
	return nil, false
}

// IsInt reports whether v is a Python int (bool excluded).
func IsInt(v any) bool {
	if _, ok := v.(bool); ok {
		return false
	}
	_, ok := BigInt(v)
	return ok
}

// NormInt returns b as an int64 when it fits and as itself otherwise,
// failing past MaxIntBits.
func NormInt(b *big.Int) (any, error) {
	if b.IsInt64() {
		return b.Int64(), nil
	}
	if b.BitLen() > MaxIntBits {
		return nil, ErrIntTooLarge
	}
	return b, nil
}

// IntToFloat converts a Python int to a float like Python's float(int):
// correctly rounded, and an OverflowError beyond the float range.
func IntToFloat(v any) (float64, error) {
	if i, ok := v.(int64); ok {
		return float64(i), nil
	}
	b, ok := BigInt(v)
	if !ok {
		return 0, errors.New("not an int")
	}
	f, _ := new(big.Float).SetInt(b).Float64()
	if math.IsInf(f, 0) {
		return 0, ErrIntToFloat
	}
	return f, nil
}

// CompareNumbers compares two Python numbers (ints, bools, floats)
// exactly, as Python does even between an int and a float: -1, 0 or 1,
// and nan when a float operand is NaN (all comparisons false). ok is
// false when either value is not a number.
func CompareNumbers(a, b any) (cmp int, nan bool, ok bool) {
	ai, aInt := intOrBool(a)
	bi, bInt := intOrBool(b)
	if aInt && bInt {
		return ai.Cmp(bi), false, true
	}
	af, aFloat := floatOf(a)
	bf, bFloat := floatOf(b)
	switch {
	case aFloat && bFloat:
		if math.IsNaN(af) || math.IsNaN(bf) {
			return 0, true, true
		}
		switch {
		case af < bf:
			return -1, false, true
		case af > bf:
			return 1, false, true
		}
		return 0, false, true
	case aInt && bFloat:
		c, nan := cmpIntFloat(ai, bf)
		return c, nan, true
	case aFloat && bInt:
		c, nan := cmpIntFloat(bi, af)
		return -c, nan, true
	}
	return 0, false, false
}

func intOrBool(v any) (*big.Int, bool) {
	if b, ok := v.(bool); ok {
		if b {
			return big.NewInt(1), true
		}
		return big.NewInt(0), true
	}
	return BigInt(v)
}

func floatOf(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	}
	return 0, false
}

// cmpIntFloat compares an int with a float exactly.
func cmpIntFloat(i *big.Int, f float64) (int, bool) {
	switch {
	case math.IsNaN(f):
		return 0, true
	case math.IsInf(f, 1):
		return -1, false
	case math.IsInf(f, -1):
		return 1, false
	}
	return new(big.Float).SetInt(i).Cmp(new(big.Float).SetFloat64(f)), false
}
