package environment

import (
	"math/rand"
	"strings"

	"github.com/jryberg/gojinja/pkg/escape"
)

// lipsumWords is a small classical-Latin word list. It's substantially
// shorter than Python's full LOREM_IPSUM_WORDS (which is 380+ entries),
// but it produces visibly varied paragraphs. Output identity with Python
// isn't a goal here — `lipsum` is a layout placeholder.
var lipsumWords = strings.Fields(`a ac accumsan ad adipiscing aenean aliquam aliquet amet ante
aptent arcu at auctor augue bibendum blandit class commodo condimentum
congue consectetur consequat conubia convallis cras cubilia curabitur
curae cursus dapibus diam dictum dictumst dignissim dis dolor donec dui
duis efficitur egestas eget eleifend elementum elit enim erat eros est
et etiam eu euismod ex facilisi facilisis fames faucibus felis fermentum
feugiat finibus fringilla fusce gravida habitant habitasse hac hendrerit
himenaeos iaculis id imperdiet in inceptos integer interdum ipsum justo
lacinia lacus laoreet lectus leo libero ligula litora lobortis lorem
luctus maecenas magna magnis malesuada massa mattis mauris maximus
metus mi molestie mollis montes morbi mus nam nascetur natoque nec
neque netus nibh nisi nisl non nostra nulla nullam nunc odio orci ornare
parturient pellentesque penatibus per pharetra phasellus placerat platea
porta porttitor posuere potenti praesent pretium primis proin pulvinar
purus quam quis quisque rhoncus ridiculus risus rutrum sagittis sapien
scelerisque sed sem semper senectus sit sociosqu sodales sollicitudin
suscipit suspendisse taciti tellus tempor tempus tincidunt torquent
tortor tristique turpis ullamcorper ultrices ultricies urna ut varius
vehicula vel velit venenatis vestibulum vitae vivamus viverra volutpat
vulputate`)

// generateLipsum builds n paragraphs of placeholder text. Each paragraph
// has between min and max words. When html is true, paragraphs are
// wrapped in <p>...</p> and the result is returned as Markup (so it
// survives autoescape).
func generateLipsum(n, min, max int, html bool) any {
	if n <= 0 {
		n = 5
	}
	if min <= 0 {
		min = 20
	}
	if max <= min {
		max = min + 80
	}
	r := rand.New(rand.NewSource(42)) // deterministic for tests

	paras := make([]string, 0, n)
	for i := 0; i < n; i++ {
		words := r.Intn(max-min) + min
		// Build sentences of 4–10 words.
		var b strings.Builder
		w := 0
		for w < words {
			sentLen := r.Intn(7) + 4
			if w+sentLen > words {
				sentLen = words - w
			}
			for j := 0; j < sentLen; j++ {
				if j > 0 {
					b.WriteByte(' ')
				}
				word := lipsumWords[r.Intn(len(lipsumWords))]
				if j == 0 {
					word = strings.ToUpper(word[:1]) + word[1:]
				}
				b.WriteString(word)
			}
			b.WriteByte('.')
			w += sentLen
			if w < words {
				b.WriteByte(' ')
			}
		}
		paras = append(paras, b.String())
	}

	if html {
		var b strings.Builder
		for _, p := range paras {
			b.WriteString("<p>")
			b.WriteString(p)
			b.WriteString("</p>\n")
		}
		return escape.Markup(strings.TrimRight(b.String(), "\n"))
	}
	return strings.Join(paras, "\n\n")
}
