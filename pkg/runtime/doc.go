// Package runtime contains the objects templates work with, mirroring
// Jinja2's runtime.py and the Python types it relies on: Context,
// EvalContext, Undefined and its variants, LoopContext, BlockReference,
// TemplateModule, Namespace, Cycler and Joiner; Python's containers and
// values — PyList, Tuple, OrderedDict (and its mappingproxy view,
// NewMappingProxy), DictView, Range, Bytes, Iterator (generators and
// reverse iterators) and Callable (methods, functions and classes) — and
// their shared semantics: HashKey, NumBinop and the other numeric
// helpers, and the missing sentinel.
package runtime
