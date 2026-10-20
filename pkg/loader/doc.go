// Package loader provides the Loader implementations: Dict, FuncLoader,
// Prefix, Choice, FileSystem (allowlist-rooted, no symlink escape), Embed
// (in place of Python's PackageLoader) and Cached. Dict, FuncLoader,
// FileSystem and Embed run template names through SplitPath before any
// lookup, and every loader reports a template it does not have as
// errors.TemplateNotFound.
package loader
