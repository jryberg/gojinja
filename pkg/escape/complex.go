package escape

import (
	"math"
	"strings"
)

// complexRepr is Python's repr of a complex: each part formatted like a
// float repr without a trailing ".0", in parentheses unless the real
// part is +0.
func complexRepr(c complex128) string {
	re, im := real(c), imag(c)
	if re == 0 && !math.Signbit(re) {
		return complexPart(im) + "j"
	}
	imPart := complexPart(im)
	if !strings.HasPrefix(imPart, "-") {
		imPart = "+" + imPart
	}
	return "(" + complexPart(re) + imPart + "j)"
}

func complexPart(f float64) string {
	return strings.TrimSuffix(pyFloatStr(f), ".0")
}
