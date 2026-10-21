package environment

import (
	"strings"

	"github.com/jryberg/gojinja/internal/pyunicode"
)

// This file ports the parts of Python's textwrap.TextWrapper that the
// wordwrap filter uses (expand_tabs=False, replace_whitespace=False, no
// indents, drop_whitespace=True). Lengths count characters, as Python's
// len() does.

// isWrapSpace is textwrap's whitespace set: '\t\n\x0b\x0c\r '.
func isWrapSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ':
		return true
	}
	return false
}

// isWrapLetter is textwrap's letter class [^\d\W].
func isWrapLetter(r rune) bool {
	return pyunicode.IsWord(r) && !pyunicode.IsDecimal(r)
}

// isWordPunct is textwrap's word_punct class [\w!"'&.,?].
func isWordPunct(r rune) bool {
	return pyunicode.IsWord(r) || strings.ContainsRune(`!"'&.,?`, r)
}

// wrapChunks splits text into chunks like TextWrapper._split: runs of
// whitespace and words, where words also split after hyphens and around
// em-dashes when breakOnHyphens is set.
func wrapChunks(text []rune, breakOnHyphens bool) [][]rune {
	var chunks [][]rune
	for i := 0; i < len(text); {
		end := i + 1
		if isWrapSpace(text[i]) {
			for end < len(text) && isWrapSpace(text[end]) {
				end++
			}
		} else if breakOnHyphens {
			end = wordsepEnd(text, i)
		} else {
			for end < len(text) && !isWrapSpace(text[end]) {
				end++
			}
		}
		chunks = append(chunks, text[i:end])
		i = end
	}
	return chunks
}

// wordsepEnd matches TextWrapper.wordsep_re at a non-whitespace position
// i and returns the end of the match. Go's regexp has no lookbehind, so
// the alternatives are tried by hand in the regex's order.
func wordsepEnd(t []rune, i int) int {
	// em-dash between words: (?<=word_punct) -{2,} (?=\w)
	if i > 0 && isWordPunct(t[i-1]) {
		if end, ok := dashRunBeforeWord(t, i); ok {
			return end
		}
	}
	// word, possibly hyphenated: \S+? followed by one of three endings,
	// trying the shortest word first.
	for end := i + 1; ; end++ {
		// hyphenated word: -(?:(?<=lt{2}-)|(?<=lt-lt-)) (?=lt -? lt)
		if end < len(t) && t[end] == '-' && hyphenAfterLetters(t, end) && letterPairAhead(t, end+1) {
			return end + 1
		}
		// end of word
		if end == len(t) || isWrapSpace(t[end]) {
			return end
		}
		// em-dash: (?<=word_punct) (?=-{2,}\w)
		if isWordPunct(t[end-1]) {
			if _, ok := dashRunBeforeWord(t, end); ok {
				return end
			}
		}
	}
}

// dashRunBeforeWord reports whether t[i:] is two or more hyphens followed
// by a word character, returning the end of the hyphens.
func dashRunBeforeWord(t []rune, i int) (int, bool) {
	j := i
	for j < len(t) && t[j] == '-' {
		j++
	}
	return j, j-i >= 2 && j < len(t) && pyunicode.IsWord(t[j])
}

// hyphenAfterLetters checks the lookbehinds for the hyphen at t[h]:
// two letters before it, or letter-hyphen-letter before it.
func hyphenAfterLetters(t []rune, h int) bool {
	if h >= 2 && isWrapLetter(t[h-1]) && isWrapLetter(t[h-2]) {
		return true
	}
	return h >= 3 && isWrapLetter(t[h-1]) && t[h-2] == '-' && isWrapLetter(t[h-3])
}

// letterPairAhead checks the lookahead (?= lt -? lt) at t[p].
func letterPairAhead(t []rune, p int) bool {
	if p >= len(t) || !isWrapLetter(t[p]) {
		return false
	}
	if p+1 < len(t) && isWrapLetter(t[p+1]) {
		return true
	}
	return p+2 < len(t) && t[p+1] == '-' && isWrapLetter(t[p+2])
}

func isBlankChunk(c []rune) bool {
	for _, r := range c {
		if !pyunicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// textWrap is textwrap.wrap(text, width, expand_tabs=False,
// replace_whitespace=False, break_long_words=..., break_on_hyphens=...).
// width must be positive.
func textWrap(text string, width int, breakLongWords, breakOnHyphens bool) []string {
	chunks := wrapChunks([]rune(text), breakOnHyphens)
	// Reverse so the next chunk is at the end, as TextWrapper does.
	for i, j := 0, len(chunks)-1; i < j; i, j = i+1, j-1 {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	}
	var lines []string
	for len(chunks) > 0 {
		var cur [][]rune
		curLen := 0
		if len(lines) > 0 && isBlankChunk(chunks[len(chunks)-1]) {
			chunks = chunks[:len(chunks)-1]
		}
		for len(chunks) > 0 {
			l := len(chunks[len(chunks)-1])
			if curLen+l > width {
				break
			}
			cur = append(cur, chunks[len(chunks)-1])
			chunks = chunks[:len(chunks)-1]
			curLen += l
		}
		if len(chunks) > 0 && len(chunks[len(chunks)-1]) > width {
			cur, chunks = handleLongWord(chunks, cur, curLen, width, breakLongWords, breakOnHyphens)
		}
		if len(cur) > 0 && isBlankChunk(cur[len(cur)-1]) {
			cur = cur[:len(cur)-1]
		}
		if len(cur) > 0 {
			var b strings.Builder
			for _, c := range cur {
				b.WriteString(string(c))
			}
			lines = append(lines, b.String())
		}
	}
	return lines
}

// handleLongWord is TextWrapper._handle_long_word.
func handleLongWord(chunks, cur [][]rune, curLen, width int, breakLongWords, breakOnHyphens bool) ([][]rune, [][]rune) {
	spaceLeft := 1
	if width >= 1 {
		spaceLeft = width - curLen
	}
	last := len(chunks) - 1
	switch {
	case breakLongWords:
		chunk := chunks[last]
		end := spaceLeft
		if breakOnHyphens && len(chunk) > spaceLeft {
			// Break after the last hyphen, if non-hyphens precede it.
			hyphen := -1
			for k := min(spaceLeft, len(chunk)) - 1; k >= 0; k-- {
				if chunk[k] == '-' {
					hyphen = k
					break
				}
			}
			if hyphen > 0 && strings.Trim(string(chunk[:hyphen]), "-") != "" {
				end = hyphen + 1
			}
		}
		cur = append(cur, chunk[:end])
		chunks[last] = chunk[end:]
	case len(cur) == 0:
		cur = append(cur, chunks[last])
		chunks = chunks[:last]
	}
	return cur, chunks
}
