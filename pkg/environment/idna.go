package environment

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// The idna codec ports Python's encodings/idna.py (IDNA2003: RFC 3490
// with RFC 3491 nameprep), with tables from tools/parity/idnatables.py.

const (
	hangulS      = 0xac00
	hangulL      = 0x1100
	hangulV      = 0x1161
	hangulT      = 0x11a7
	hangulLCount = 19
	hangulVCount = 21
	hangulTCount = 28
	hangulNCount = hangulVCount * hangulTCount
	hangulSCount = hangulLCount * hangulNCount
)

// nfkc32 is unicodedata.ucd_3_2_0.normalize("NFKC", s) as CPython runs
// it: decomposition with Unicode 3.2 data, then canonical reordering and
// composition (nfd_nfkd and nfc_nfkc in unicodedata.c) with the current
// database's combining classes and pairs.
func nfkc32(rs []rune) []rune {
	var d []rune
	for _, r := range rs {
		switch {
		case r >= hangulS && r < hangulS+hangulSCount:
			s := r - hangulS
			d = append(d, hangulL+s/hangulNCount, hangulV+s%hangulNCount/hangulTCount)
			if s%hangulTCount != 0 {
				d = append(d, hangulT+s%hangulTCount)
			}
		case nfkd32[r] != "":
			d = append(d, []rune(nfkd32[r])...)
		default:
			d = append(d, r)
		}
	}
	// Canonical reordering: an insertion sort within runs of non-starters.
	for i := 1; i < len(d); i++ {
		cur := combiningClass[d[i]]
		for o := i - 1; o >= 0 && cur != 0; o-- {
			prev := combiningClass[d[o]]
			if prev == 0 || prev <= cur {
				break
			}
			d[o], d[o+1] = d[o+1], d[o]
		}
	}
	out := make([]rune, 0, len(d))
	skipped := map[int]bool{}
	for i := 0; i < len(d); {
		if skipped[i] {
			delete(skipped, i)
			i++
			continue
		}
		code := d[i]
		if code >= hangulL && code < hangulL+hangulLCount && i+1 < len(d) &&
			d[i+1] >= hangulV && d[i+1] < hangulV+hangulVCount {
			code = hangulS + ((code-hangulL)*hangulVCount+d[i+1]-hangulV)*hangulTCount
			i += 2
			if i < len(d) && d[i] > hangulT && d[i] < hangulT+hangulTCount {
				code += d[i] - hangulT
				i++
			}
			out = append(out, code)
			continue
		}
		if !composeFirst[code] {
			out = append(out, code)
			i++
			continue
		}
		// Combine with each following unblocked character that pairs.
		var comb uint8
		for i1 := i + 1; i1 < len(d); i1++ {
			code1 := d[i1]
			comb1 := combiningClass[code1]
			if comb != 0 {
				if comb1 == 0 {
					break
				}
				if comb >= comb1 {
					continue
				}
			}
			composite, ok := rune(0), false
			if composeLast[code1] {
				composite, ok = composePairs[[2]rune{code, code1}]
			}
			if !ok {
				if comb1 == 0 {
					break
				}
				comb = comb1
				continue
			}
			code = composite
			skipped[i1] = true
			if !composeFirst[code] {
				break
			}
		}
		out = append(out, code)
		i++
	}
	return out
}

// composeFirst and composeLast hold the characters that start and end a
// composition pair.
var composeFirst, composeLast = composeSets()

func composeSets() (first, last map[rune]bool) {
	first, last = map[rune]bool{}, map[rune]bool{}
	for pair := range composePairs {
		first[pair[0]] = true
		last[pair[1]] = true
	}
	return first, last
}

// nameprep is RFC 3491 with AllowUnassigned, as encodings.idna.nameprep.
func nameprep(label []rune) ([]rune, error) {
	var mapped []rune
	for _, r := range label {
		if inRanges(nameprepB1, r) {
			continue
		}
		if m, ok := nameprepB2[r]; ok {
			mapped = append(mapped, []rune(m)...)
		} else {
			mapped = append(mapped, r)
		}
	}
	label = nfkc32(mapped)
	for _, r := range label {
		if inRanges(nameprepProhibited, r) {
			//lint:ignore ST1005 Python's exact message
			return nil, fmt.Errorf("Invalid character %s", escape.Repr(string(r)))
		}
	}
	randAL := false
	for _, r := range label {
		randAL = randAL || inRanges(nameprepRandAL, r)
	}
	if randAL {
		for _, r := range label {
			if inRanges(nameprepL, r) {
				//lint:ignore ST1005 Python's exact message
				return nil, fmt.Errorf("Violation of BIDI requirement 2")
			}
		}
		if !inRanges(nameprepRandAL, label[0]) || !inRanges(nameprepRandAL, label[len(label)-1]) {
			//lint:ignore ST1005 Python's exact message
			return nil, fmt.Errorf("Violation of BIDI requirement 3")
		}
	}
	return label, nil
}

func isASCIIRunes(rs []rune) bool {
	for _, r := range rs {
		if r >= 0x80 {
			return false
		}
	}
	return true
}

var errLabelLength = fmt.Errorf("label empty or too long")

// idnaToASCII is RFC 3490 ToASCII with UseSTD3ASCIIRules false.
func idnaToASCII(label []rune) ([]byte, error) {
	if !isASCIIRunes(label) {
		var err error
		if label, err = nameprep(label); err != nil {
			return nil, err
		}
	}
	var out []byte
	if isASCIIRunes(label) {
		out = []byte(string(label))
	} else {
		if strings.HasPrefix(string(label), "xn--") {
			//lint:ignore ST1005 Python's exact message
			return nil, fmt.Errorf("Label starts with ACE prefix")
		}
		out = append([]byte("xn--"), punycodeEncode(label)...)
	}
	if len(out) == 0 || len(out) >= 64 {
		return nil, errLabelLength
	}
	return out, nil
}

// idnaToUnicode is RFC 3490 ToUnicode for a label of the bytes being
// decoded.
func idnaToUnicode(label []byte) (string, error) {
	if len(label) > 1024 {
		return "", fmt.Errorf("label way too long")
	}
	if !bytes.HasPrefix(label, []byte("xn--")) {
		return asciiStrict(label)
	}
	result, err := punycodeDecode(label[4:], "strict")
	if err != nil {
		return "", err
	}
	label2, err := idnaToASCII([]rune(result))
	if err != nil {
		return "", err
	}
	if strings.ToLower(string(label)) != string(label2) {
		return "", fmt.Errorf("('IDNA does not round-trip', %s, %s)", runtime.Bytes(label), runtime.Bytes(label2))
	}
	return result, nil
}

// asciiStrict is str(b, "ascii").
func asciiStrict(b []byte) (string, error) {
	for k, c := range b {
		if c >= 0x80 {
			return "", fmt.Errorf("'ascii' codec can't decode byte 0x%02x in position %d: ordinal not in range(128)", c, k)
		}
	}
	return string(b), nil
}

func isIDNADot(r rune) bool { return r == '.' || r == '。' || r == '．' || r == '｡' }

func idnaEncode(s, errorsMode string) (runtime.Bytes, error) {
	if errorsMode != "strict" {
		return nil, fmt.Errorf("unsupported error handling %s", errorsMode)
	}
	if s == "" {
		return runtime.Bytes{}, nil
	}
	rs := []rune(s)
	if isASCIIRunes(rs) {
		labels := strings.Split(s, ".")
		for _, label := range labels[:len(labels)-1] {
			if len(label) == 0 || len(label) >= 64 {
				return nil, errLabelLength
			}
		}
		if len(labels[len(labels)-1]) >= 64 {
			return nil, fmt.Errorf("label too long")
		}
		return runtime.Bytes(s), nil
	}
	var labels [][]rune
	start := 0
	for i, r := range rs {
		if isIDNADot(r) {
			labels = append(labels, rs[start:i])
			start = i + 1
		}
	}
	labels = append(labels, rs[start:])
	trailingDot := false
	if len(labels[len(labels)-1]) == 0 {
		trailingDot = true
		labels = labels[:len(labels)-1]
	}
	out := runtime.Bytes{}
	for _, label := range labels {
		if len(out) > 0 {
			out = append(out, '.')
		}
		b, err := idnaToASCII(label)
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	if trailingDot {
		out = append(out, '.')
	}
	return out, nil
}

func idnaDecode(b []byte, errorsMode string) (string, error) {
	if errorsMode != "strict" {
		//lint:ignore ST1005 Python's exact message
		return "", fmt.Errorf("Unsupported error handling %s", errorsMode)
	}
	if !bytes.Contains(b, []byte("xn--")) {
		if s, err := asciiStrict(b); err == nil {
			return s, nil
		}
	}
	labels := bytes.Split(b, []byte("."))
	trailingDot := ""
	if len(labels[len(labels)-1]) == 0 {
		trailingDot = "."
		labels = labels[:len(labels)-1]
	}
	parts := make([]string, len(labels))
	for i, label := range labels {
		s, err := idnaToUnicode(label)
		if err != nil {
			return "", err
		}
		parts[i] = s
	}
	return strings.Join(parts, ".") + trailingDot, nil
}
