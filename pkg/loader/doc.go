// Package loader provides Loader implementations: DictLoader, FunctionLoader,
// PrefixLoader, ChoiceLoader, FileSystemLoader (allowlist-rooted, no symlink
// escape), EmbedLoader (replaces Python's PackageLoader), and ModuleLoader
// (pre-compiled IR). All loaders run user-supplied template names through
// the splitTemplatePath validator before any I/O.
package loader
