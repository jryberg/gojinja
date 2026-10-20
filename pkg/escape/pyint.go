package escape

import (
	"fmt"
	"math/big"
)

// MaxStrDigits is Python's default sys.get_int_max_str_digits(): the
// most decimal digits an int may be converted to or from.
const MaxStrDigits = 4300

// ValueError is what [Repr] and [SoftStr] panic with for an int too long
// to print in decimal, as they cannot return an error; the evaluator
// recovers it and fails the render with it.
type ValueError struct{ Msg string }

func (e ValueError) Error() string { return e.Msg }

const errIntDigits = "Exceeds the limit (4300 digits) for integer string conversion; use sys.set_int_max_str_digits() to increase the limit"

// IntDecimal is str(b): its decimal digits, or Python's ValueError past
// MaxStrDigits.
func IntDecimal(b *big.Int) (string, error) {
	// More than MaxStrDigits*log2(10) bits always has too many digits;
	// skip formatting such a value.
	if b.BitLen() > MaxStrDigits*10/3+10 {
		return "", ValueError{errIntDigits}
	}
	s := b.String()
	digits := len(s)
	if b.Sign() < 0 {
		digits--
	}
	if digits > MaxStrDigits {
		return "", ValueError{errIntDigits}
	}
	return s, nil
}

// ParseIntDigits checks a decimal digit string's length the way Python's
// int(str) does before converting it.
func ParseIntDigits(digits int) error {
	if digits > MaxStrDigits {
		return ValueError{fmt.Sprintf("Exceeds the limit (4300 digits) for integer string conversion: value has %d digits; use sys.set_int_max_str_digits() to increase the limit", digits)}
	}
	return nil
}

func intRepr(b *big.Int) string {
	s, err := IntDecimal(b)
	if err != nil {
		panic(err)
	}
	return s
}
