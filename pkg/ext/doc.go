// Package ext provides the opt-in extensions for gojinja: i18n
// (`{% trans %}` / `{% pluralize %}` / `{{ _(...) }}` backed by a
// [Translator]) and debug (`{% debug %}`).
//
// The first-party extensions Python Jinja2 ships as opt-in — `do`,
// `break`/`continue`, `with`, and the `autoescape` block — are built
// into the gojinja parser itself rather than living here, so they're
// always available. See the [package divergences].
//
// To enable an extension on an Environment, register the parser hook and
// any required globals at construction:
//
//	env, _ := environment.New(
//	    environment.WithExtension("trans", ext.TransTag),
//	    environment.WithGlobals(ext.TranslatorGlobals(myTr)),
//	)
//
// [package divergences]: https://github.com/jryberg/gojinja/blob/main/docs/divergences.md
package ext
