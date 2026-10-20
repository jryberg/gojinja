package environment

import (
	"math/rand/v2"
	"strings"

	"github.com/jryberg/gojinja/pkg/escape"
)

// lipsumSignature is generate_lorem_ipsum's.
var lipsumSignature = &signature{fn: "generate_lorem_ipsum", params: []string{"n", "html", "min", "max"}}

// generateLipsum is jinja2.utils.generate_lorem_ipsum: n paragraphs of
// min to max-1 words from Jinja2's word list, with commas and sentences
// placed the same random way, as Markup `<p>` paragraphs or, without html,
// plain text with blank lines between them.
func generateLipsum(n int, html bool, min, max any) (any, error) {
	var result []string
	for range max0(n) {
		count, err := randrange(min, max)
		if err != nil {
			return nil, err
		}
		nextCapitalized := true
		lastComma, lastFullstop := 0, 0
		last := ""
		var p []string
		for idx := range max0(count) {
			word := lipsumWords[rand.IntN(len(lipsumWords))]
			for word == last {
				word = lipsumWords[rand.IntN(len(lipsumWords))]
			}
			last = word
			if nextCapitalized {
				word = pyCapitalize(word)
				nextCapitalized = false
			}
			// add commas
			if idx-(3+rand.IntN(5)) > lastComma {
				lastComma = idx
				lastFullstop += 2
				word += ","
			}
			// add end of sentences
			if idx-(10+rand.IntN(10)) > lastFullstop {
				lastComma, lastFullstop = idx, idx
				word += "."
				nextCapitalized = true
			}
			p = append(p, word)
		}
		// ensure that the paragraph ends with a dot.
		para := strings.Join(p, " ")
		if strings.HasSuffix(para, ",") {
			para = para[:len(para)-1] + "."
		} else if !strings.HasSuffix(para, ".") {
			para += "."
		}
		result = append(result, para)
	}
	if !html {
		return strings.Join(result, "\n\n"), nil
	}
	paras := make([]string, len(result))
	for i, para := range result {
		paras[i] = "<p>" + string(escape.Escape(para)) + "</p>"
	}
	return escape.Markup(strings.Join(paras, "\n")), nil
}

// randrange is Python's random.randrange(start, stop).
func randrange(startV, stopV any) (int, error) {
	start, err := indexArg(startV)
	if err != nil {
		return 0, err
	}
	stop, err := indexArg(stopV)
	if err != nil {
		return 0, err
	}
	if stop <= start {
		return 0, methodError("empty range in randrange(%d, %d)", start, stop)
	}
	return start + rand.IntN(stop-start), nil
}

func max0(n int) int {
	return max(n, 0)
}
