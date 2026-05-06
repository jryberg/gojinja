# Symbols index

Generated from godoc. Each row links to the symbol's reference on
pkg.go.dev, pinned to **latest**. Set `MIKE_VERSION` to repin.


## `github.com/jryberg/gojinja`

| Symbol | Kind | Synopsis | Reference |
|---|---|---|---|
| `AutoescapeAlways` | type | AutoescapeAlways enables autoescape for every template. The default. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#AutoescapeAlways) |
| `AutoescapeByExtension` | type | AutoescapeByExtension enables autoescape based on the template name's | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#AutoescapeByExtension) |
| `AutoescapeNever` | type | AutoescapeNever disables autoescape entirely. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#AutoescapeNever) |
| `AutoescapePolicy` | type | AutoescapePolicy decides whether to autoescape a given template. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#AutoescapePolicy) |
| `Cache` | type | Cache is the persistent AST cache contract. Pass an implementation to | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#Cache) |
| `CachedLoader` | type | CachedLoader memoises a Loader's results in process. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#CachedLoader) |
| `ChoiceLoader` | type | ChoiceLoader tries each loader in order, returning the first hit. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#ChoiceLoader) |
| `DictLoader` | type | DictLoader is an in-memory map of name→source. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#DictLoader) |
| `EmbedLoader` | type | EmbedLoader reads templates from an [embed.FS] under a path prefix. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#EmbedLoader) |
| `Environment` | type | Environment is the central configuration object. Construct with [New]. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#Environment) |
| `FileSystemLoader` | type | FileSystemLoader reads templates from one or more allowlisted directories. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#FileSystemLoader) |
| `FilesystemCache` | type | FilesystemCache is an on-disk AST cache (atomic writes, magic+checksum). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#FilesystemCache) |
| `FuncLoader` | type | FuncLoader wraps an arbitrary fetch function. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#FuncLoader) |
| `Loader` | type | Loader is the contract a template-source provider implements (env-side). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#Loader) |
| `Markup` | type | Markup re-exports [pkg/escape.Markup] for callers that want to mark | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#Markup) |
| `MemoryCache` | type | MemoryCache is an in-process LRU-bounded AST cache. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#MemoryCache) |
| `Namespace` | type | Namespace is the runtime container for `&#123;% set ns.attr = ... %}` writes. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#Namespace) |
| `New` | func | New constructs an Environment with safe defaults. Pass [Option]s to | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#New) |
| `NewCachedLoader` | var | NewCachedLoader wraps an existing Loader with in-memory memoisation. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#NewCachedLoader) |
| `NewEmbedLoader` | var | NewEmbedLoader builds an [EmbedLoader] over fsys rooted at prefix. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#NewEmbedLoader) |
| `NewFileSystemLoader` | var | NewFileSystemLoader builds a [FileSystemLoader] over the given roots. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#NewFileSystemLoader) |
| `NewFilesystemCache` | var | NewFilesystemCache constructs a FilesystemCache rooted at dir. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#NewFilesystemCache) |
| `NewMemoryCache` | var | NewMemoryCache constructs a MemoryCache. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#NewMemoryCache) |
| `Option` | type | Option configures an [Environment] at construction. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#Option) |
| `PrefixLoader` | type | PrefixLoader routes templates by name prefix to sub-loaders. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#PrefixLoader) |
| `Source` | type | Source is a loaded template's source text plus metadata. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#Source) |
| `Template` | type | Template is a compiled template ready to render. Returned by | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#Template) |
| `Undefined` | type | Undefined is the default undefined-variant. Other variants live in | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#Undefined) |
| `WithAutoescape` | var | WithAutoescape replaces the autoescape policy (default: [AutoescapeAlways]). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithAutoescape) |
| `WithCacheSize` | var | WithCacheSize bounds the in-memory template cache. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithCacheSize) |
| `WithExternalCache` | var | WithExternalCache plugs a persistent AST cache (typically a [cache.Filesystem]). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithExternalCache) |
| `WithFilter` | var | WithFilter registers a filter under a name. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithFilter) |
| `WithGlobal` | var | WithGlobal registers (or replaces) a global by name. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithGlobal) |
| `WithHostEnv` | var | WithHostEnv permits host environment-variable access. Off by default. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithHostEnv) |
| `WithLexerOptions` | var | WithLexerOptions tweaks lexer-level settings (block markers, whitespace, etc.). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithLexerOptions) |
| `WithLoader` | var | WithLoader installs a [Loader]. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithLoader) |
| `WithRangeLimit` | var | WithRangeLimit caps the size of any single `range()` call. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithRangeLimit) |
| `WithTest` | var | WithTest registers a test under a name. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithTest) |
| `WithUndefined` | var | WithUndefined chooses the undefined-variant produced for missing names. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithUndefined) |
| `WithUnsafe` | var | WithUnsafe disables the default sandbox. Reserved for trusted templates; | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja#WithUnsafe) |

## `github.com/jryberg/gojinja/pkg/ast`

| Symbol | Kind | Synopsis | Reference |
|---|---|---|---|
| `Add` | type | Add — `a + b`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Add) |
| `And` | type | And — `a and b`. Short-circuits. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#And) |
| `Assign` | type | Assign is `&#123;% set target = expr %}`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Assign) |
| `AssignBlock` | type | AssignBlock is `&#123;% set target %} body &#123;% endset %}`, optionally with a | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#AssignBlock) |
| `AssignContext` | type | AssignContext is the role a [Name] / [Tuple] plays. The parser builds | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#AssignContext) |
| `Block` | type | Block is `&#123;% block name %} body &#123;% endblock %}`. The Scoped and Required | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Block) |
| `Break` | type | Break is the `&#123;% break %}` directive (loopcontrols extension). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Break) |
| `Call` | type | Call is a callable invocation: `obj(arg, kw=v, *a, **k)`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Call) |
| `CallBlock` | type | CallBlock is `&#123;% call name(args) %} body &#123;% endcall %}`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#CallBlock) |
| `Children` | func | Children returns every direct child node of n. The result is appended | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Children) |
| `Compare` | type | Compare implements chained comparisons: `a < b == c`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Compare) |
| `Concat` | type | Concat is the result of `a ~ b ~ c` (string-concat operator). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Concat) |
| `CondExpr` | type | CondExpr is the inline-if expression: `a if test else b`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#CondExpr) |
| `Const` | type | Const wraps a Go-side constant value: int, float64, string, bool, nil, | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Const) |
| `ContextReference` | type | ContextReference yields the active runtime context. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#ContextReference) |
| `Continue` | type | Continue is the `&#123;% continue %}` directive (loopcontrols extension). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Continue) |
| `DerivedContextReference` | type | DerivedContextReference yields the active context including local vars. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#DerivedContextReference) |
| `Dict` | type | Dict is a dict literal: `{a: 1, b: 2}`. Items are [Pair] nodes. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Dict) |
| `Div` | type | Div — `a / b` (Python true division: int/int = float). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Div) |
| `Dump` | func | Dump produces a human-readable representation of the AST useful for | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Dump) |
| `EnvironmentAttribute` | type | EnvironmentAttribute reads a named attribute from the Environment. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#EnvironmentAttribute) |
| `ErrImpossible` | var | ErrImpossible signals that a node could not be folded to a constant. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#ErrImpossible) |
| `EvalContextModifier` | type | EvalContextModifier flips eval-context flags (autoescape, etc.). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#EvalContextModifier) |
| `EvalCtx` | type | EvalCtx is the slice of evaluation context that constant folding cares | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#EvalCtx) |
| `Expr` | type | Expr is the marker for expression nodes. Every Expr supports [AsConst] | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Expr) |
| `ExprStmt` | type | ExprStmt evaluates an expression and discards the result — used by the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#ExprStmt) |
| `Extends` | type | Extends represents `&#123;% extends template %}`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Extends) |
| `ExtensionAttribute` | type | ExtensionAttribute reads a constant from a registered extension. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#ExtensionAttribute) |
| `Filter` | type | Filter applies `expr \| name(args)`. When Node is nil the filter is the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Filter) |
| `FilterBlock` | type | FilterBlock is `&#123;% filter name(args) %} body &#123;% endfilter %}`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#FilterBlock) |
| `FloorDiv` | type | FloorDiv — `a // b`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#FloorDiv) |
| `For` | type | For is the `&#123;% for ... %}` loop. Test is the optional inline filter | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#For) |
| `FromImport` | type | FromImport is `&#123;% from template import name1, name2 as alias [with/without context] %}`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#FromImport) |
| `Getattr` | type | Getattr is attribute access: `obj.attr`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Getattr) |
| `Getitem` | type | Getitem is subscript access: `obj[arg]`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Getitem) |
| `Helper` | type | Helper is the marker for non-Stmt, non-Expr nodes like [Pair], [Keyword], | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Helper) |
| `If` | type | If is `&#123;% if test %} body ... &#123;% elif ... %} ... &#123;% else %} ... &#123;% endif %}`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#If) |
| `Import` | type | Import is `&#123;% import template as target [with/without context] %}`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Import) |
| `ImportName` | type | ImportName is one entry in [FromImport.Names]. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#ImportName) |
| `ImportedName` | type | ImportedName is a placeholder referencing an imported Go function. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#ImportedName) |
| `Include` | type | Include is `&#123;% include template [with/without context] [ignore missing] %}`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Include) |
| `InternalName` | type | InternalName is a parser-generated identifier hidden from templates. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#InternalName) |
| `IsImpossible` | func | IsImpossible reports whether err originates from a folder bailing out. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#IsImpossible) |
| `Keyword` | type | Keyword is a (name, value) entry passed as a keyword argument to a Call/Filter/Test. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Keyword) |
| `List` | type | List is a list literal: `[1, 2, 3]`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#List) |
| `Literal` | type | Literal is the marker for literal expressions: Const, TemplateData, | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Literal) |
| `Macro` | type | Macro is `&#123;% macro name(args, defaults) %} body &#123;% endmacro %}`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Macro) |
| `MarkSafe` | type | MarkSafe wraps an expression as Markup unconditionally. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#MarkSafe) |
| `MarkSafeIfAutoescape` | type | MarkSafeIfAutoescape wraps as Markup only when autoescape is active. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#MarkSafeIfAutoescape) |
| `Mod` | type | Mod — `a % b`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Mod) |
| `Mul` | type | Mul — `a * b`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Mul) |
| `NSRef` | type | NSRef is a namespace assignment target: `ns.attr` on the left of `=`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#NSRef) |
| `Name` | type | Name is a variable reference: `foo`, with a load/store/param context. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Name) |
| `Neg` | type | Neg — `-x`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Neg) |
| `Node` | type | Node is the marker every AST node implements. Implementations are | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Node) |
| `Not` | type | Not — `not x`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Not) |
| `Operand` | type | Operand pairs an operator with its right-hand expression inside a | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Operand) |
| `Or` | type | Or — `a or b`. Short-circuits. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Or) |
| `Output` | type | Output is a "print" group: a sequence of expressions that all render to | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Output) |
| `OverlayScope` | type | OverlayScope inserts a context dict into a sub-scope. Used by | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#OverlayScope) |
| `Pair` | type | Pair is a (key, value) entry inside a [Dict]. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Pair) |
| `Pos` | type | Pos is the source position attached to every Node. Line numbers are | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Pos) |
| `Pow` | type | Pow — `a ** b`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Pow) |
| `Scope` | type | Scope is an artificial scope the compiler may introduce; it has no | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Scope) |
| `ScopedEvalContextModifier` | type | ScopedEvalContextModifier modifies the eval context within a body and | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#ScopedEvalContextModifier) |
| `Slice` | type | Slice is the `start:stop:step` argument inside a subscript. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Slice) |
| `SliceVal` | type | SliceVal is the folded form of a [Slice]. Each field is nil if absent. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#SliceVal) |
| `Stmt` | type | Stmt is the marker for statement nodes (ones that appear in a list of | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Stmt) |
| `Sub` | type | Sub — `a - b`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Sub) |
| `Template` | type | Template is the outermost wrapper node — every parsed source becomes a | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Template) |
| `TemplateData` | type | TemplateData is the literal text between tags. Auto-escape rules apply: | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#TemplateData) |
| `Test` | type | Test applies `expr is name(args)`. Same shape as Filter. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Test) |
| `Tuple` | type | Tuple is a tuple literal: `(1, 2, 3)` or the implicit form `1, 2, 3`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Tuple) |
| `UAdd` | type | UAdd — `+x` (unary plus). No-op for numerics. Named to avoid collision | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#UAdd) |
| `Visitor` | type | Visitor is the contract for tree walkers. Visit is called for every | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Visitor) |
| `VisitorFunc` | type | VisitorFunc adapts a plain function to [Visitor]. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#VisitorFunc) |
| `Walk` | func | Walk visits every node in the subtree rooted at n in depth-first | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#Walk) |
| `With` | type | With is `&#123;% with x=1, y=2 %} body &#123;% endwith %}`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ast#With) |

## `github.com/jryberg/gojinja/pkg/cache`

| Symbol | Kind | Synopsis | Reference |
|---|---|---|---|
| `Cache` | type | Cache is the storage interface. Implementations must be safe for | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/cache#Cache) |
| `CacheKey` | func | CacheKey returns a stable filesystem-safe key for (name, filename). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/cache#CacheKey) |
| `Filesystem` | type | Filesystem stores cache entries under a directory using atomic | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/cache#Filesystem) |
| `Memory` | type | Memory is an in-process bounded cache. Eviction is naive (drops an | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/cache#Memory) |
| `NewFilesystem` | func | NewFilesystem returns a Filesystem cache rooted at dir, creating dir | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/cache#NewFilesystem) |
| `NewMemory` | func | NewMemory returns a Memory with the given capacity. Capacity ≤ 0 is | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/cache#NewMemory) |
| `SourceChecksum` | func | SourceChecksum returns the canonical hex SHA-256 of source. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/cache#SourceChecksum) |

## `github.com/jryberg/gojinja/pkg/environment`

| Symbol | Kind | Synopsis | Reference |
|---|---|---|---|
| `AutoescapeAlways` | type | AutoescapeAlways always returns true. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#AutoescapeAlways) |
| `AutoescapeByExtension` | type | AutoescapeByExtension is a [AutoescapePolicy] backed by a list of | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#AutoescapeByExtension) |
| `AutoescapeNever` | type | AutoescapeNever always returns false. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#AutoescapeNever) |
| `AutoescapePolicy` | type | AutoescapePolicy decides whether to autoescape a given template. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#AutoescapePolicy) |
| `Environment` | type | Environment is the central object. It holds lexer/parser settings, | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#Environment) |
| `Filter` | type | Filter is a registered filter with its dispatch hint. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#Filter) |
| `FilterFunc` | type | FilterFunc is a registered template filter. The pass-arg dispatch is | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#FilterFunc) |
| `KwargsCallable` | type | KwargsCallable is the signature globals / filters / tests use when | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#KwargsCallable) |
| `Loader` | type | Loader is re-exported from [pkg/loader]. A loader resolves a template | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#Loader) |
| `New` | func | New constructs an Environment with safe defaults and the given options. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#New) |
| `Option` | type | Option configures an [Environment] at construction. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#Option) |
| `Source` | type | Source is re-exported from [pkg/loader]. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#Source) |
| `Template` | type | Template is a compiled template ready to render. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#Template) |
| `Test` | type | Test is a registered test with its dispatch hint. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#Test) |
| `TestFunc` | type | TestFunc is a registered template test (predicate). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#TestFunc) |
| `WithAutoescape` | func | WithAutoescape replaces the autoescape policy. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithAutoescape) |
| `WithCacheSize` | func | WithCacheSize sets the maximum compiled template cache. Default 400. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithCacheSize) |
| `WithDebugExtension` | func | WithDebugExtension enables the `&#123;% debug %}` tag, dumping the active | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithDebugExtension) |
| `WithExtension` | func | WithExtension registers a parser extension. parse is invoked when the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithExtension) |
| `WithExternalCache` | func | WithExternalCache plugs a persistent AST cache (typically a | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithExternalCache) |
| `WithFilter` | func | WithFilter registers a filter under name. Pass=PassNone means the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithFilter) |
| `WithGlobal` | func | WithGlobal registers (or replaces) a global by name. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithGlobal) |
| `WithHostEnv` | func | WithHostEnv registers `env` as a global function that returns the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithHostEnv) |
| `WithI18NExtension` | func | WithI18NExtension enables `&#123;% trans %}` / `&#123;% pluralize %}` / | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithI18NExtension) |
| `WithLexerOptions` | func | WithLexerOptions overrides lexer settings (block markers, whitespace, etc.). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithLexerOptions) |
| `WithLoader` | func | WithLoader sets the template loader. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithLoader) |
| `WithRangeLimit` | func | WithRangeLimit sets the maximum range size. Default 100000. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithRangeLimit) |
| `WithTest` | func | WithTest registers a test under name. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithTest) |
| `WithUndefined` | func | WithUndefined chooses the [Undefined] variant produced for missing | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithUndefined) |
| `WithUnsafe` | func | WithUnsafe disables the default sandbox. Reserved for explicit | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/environment#WithUnsafe) |

## `github.com/jryberg/gojinja/pkg/escape`

| Symbol | Kind | Synopsis | Reference |
|---|---|---|---|
| `Concat` | func | Concat joins values into a single Markup. Plain values are escaped; Markup | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/escape#Concat) |
| `Escape` | func | Escape returns v as a [Markup]. If v is already safe (Markup or | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/escape#Escape) |
| `ForceEscape` | func | ForceEscape always escapes — even values that are already [Markup]. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/escape#ForceEscape) |
| `HTMLer` | type | HTMLer is the contract for values that already represent safe HTML. The | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/escape#HTMLer) |
| `IsMarkup` | func | IsMarkup reports whether v is a [Markup] value or implements [HTMLer], and | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/escape#IsMarkup) |
| `Markup` | type | Markup is a string the engine has marked as already-safe for HTML | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/escape#Markup) |
| `PlainConcat` | func | PlainConcat joins values into a plain string, calling [SoftStr] on each. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/escape#PlainConcat) |
| `SoftStr` | func | SoftStr returns the string form of v *without* escaping. It mirrors | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/escape#SoftStr) |

## `github.com/jryberg/gojinja/pkg/ext`

| Symbol | Kind | Synopsis | Reference |
|---|---|---|---|
| `DebugFunc` | type | DebugFunc is the signature of the helper invoked by `&#123;% debug %}`. It | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#DebugFunc) |
| `DebugGlobalName` | const | DebugGlobalName is the synthetic global the &#123;% debug %} tag expands to. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#DebugGlobalName) |
| `DebugRender` | func | DebugRender is the default helper bound by [DebugGlobalName]. It dumps | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#DebugRender) |
| `DebugTag` | func | DebugTag is the parser hook for `&#123;% debug %}`. It expands to an | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#DebugTag) |
| `EnvLister` | type | EnvLister is the slice of *environment.Environment behaviour the debug | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#EnvLister) |
| `I18NGettextName` | const | I18N global names. They're registered as plain Go funcs on the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#I18NGettextName) |
| `I18NNGettextName` | const | I18N global names. They're registered as plain Go funcs on the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#I18NNGettextName) |
| `I18NNPGettextName` | const | I18N global names. They're registered as plain Go funcs on the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#I18NNPGettextName) |
| `I18NPGettextName` | const | I18N global names. They're registered as plain Go funcs on the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#I18NPGettextName) |
| `I18NSubst` | func | I18NSubst replaces `%(name)s` placeholders in s with the stringified | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#I18NSubst) |
| `I18NSubstName` | const | I18N global names. They're registered as plain Go funcs on the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#I18NSubstName) |
| `I18NTag` | func | I18NTag is the parser hook for `&#123;% trans %}`. It supports: | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#I18NTag) |
| `I18NUnderscoreAlias` | const | I18N global names. They're registered as plain Go funcs on the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#I18NUnderscoreAlias) |
| `NullTranslator` | type | NullTranslator is the passthrough [Translator]: returns messages | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#NullTranslator) |
| `Translator` | type | Translator is the gettext-style backend the i18n extension calls into. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#Translator) |
| `TranslatorGlobals` | func | TranslatorGlobals returns the gettext-family functions backed by t. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/ext#TranslatorGlobals) |

## `github.com/jryberg/gojinja/pkg/loader`

| Symbol | Kind | Synopsis | Reference |
|---|---|---|---|
| `Cached` | type | Cached wraps a Loader and memoises results in process. Useful when | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#Cached) |
| `Choice` | type | Choice tries each loader in order, returning the first hit. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#Choice) |
| `Dict` | type | Dict is a string→source map. The simplest possible loader — useful for | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#Dict) |
| `Embed` | type | Embed wraps an `fs.FS` (typically an [embed.FS]) under a path prefix. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#Embed) |
| `ErrInvalidPath` | var | ErrInvalidPath is returned by SplitPath for unsafe template names. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#ErrInvalidPath) |
| `FileSystem` | type | FileSystem reads templates from one or more allowlisted root directories. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#FileSystem) |
| `FileSystemOption` | type | FileSystemOption configures a [FileSystem] loader. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#FileSystemOption) |
| `FuncLoader` | type | FuncLoader wraps an arbitrary fetch function. Useful for adapting to | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#FuncLoader) |
| `Loader` | type | Loader is the in-package interface every loader implements. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#Loader) |
| `NewCached` | func | NewCached returns a Cached over inner. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#NewCached) |
| `NewEmbed` | func | NewEmbed returns an Embed loader rooted at prefix inside fsys. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#NewEmbed) |
| `NewFileSystem` | func | NewFileSystem builds a FileSystem loader over the given root(s). Roots | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#NewFileSystem) |
| `Prefix` | type | Prefix routes by name prefix. `mapping["app1"] = …` makes "app1/x.html" | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#Prefix) |
| `Source` | type | Source is the result of resolving a template name. The environment | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#Source) |
| `SplitPath` | func | SplitPath validates a template name and returns its path components. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#SplitPath) |
| `WithFollowLinks` | func | WithFollowLinks permits resolving symlinks. Off by default — when off, | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/loader#WithFollowLinks) |

## `github.com/jryberg/gojinja/pkg/meta`

| Symbol | Kind | Synopsis | Reference |
|---|---|---|---|
| `FindReferencedTemplates` | func | FindReferencedTemplates returns the names of templates the template | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/meta#FindReferencedTemplates) |
| `FindUndeclaredVariables` | func | FindUndeclaredVariables returns the set of names the template will try | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/meta#FindUndeclaredVariables) |

## `github.com/jryberg/gojinja/pkg/native`

| Symbol | Kind | Synopsis | Reference |
|---|---|---|---|
| `Concat` | func | Concat parses s as a JSON literal. Returns a typed Go value when | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/native#Concat) |
| `Render` | func | Render produces a typed Go value for tpl. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/native#Render) |

## `github.com/jryberg/gojinja/pkg/runtime`

| Symbol | Kind | Synopsis | Reference |
|---|---|---|---|
| `BlockRenderFunc` | type | BlockRenderFunc renders one level of a block in the inheritance chain. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#BlockRenderFunc) |
| `CallerFunc` | type | CallerFunc is the runtime contract for `caller()` produced by the | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#CallerFunc) |
| `Context` | type | Context carries the per-render variable bindings and inheritance state. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#Context) |
| `Cycler` | type | Cycler cycles through a fixed list of values, returning the next value | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#Cycler) |
| `EvalContext` | type | EvalContext holds eval-time flags that can be flipped by AST nodes during | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#EvalContext) |
| `EvalContextEnvironment` | type | EvalContextEnvironment is the slice of the Environment that an EvalContext | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#EvalContextEnvironment) |
| `IsMissing` | func | IsMissing returns true iff v is the [Missing] sentinel. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#IsMissing) |
| `IsUndefined` | func | IsUndefined reports whether v is an Undefined value (any mode). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#IsUndefined) |
| `Joiner` | type | Joiner emits a separator on every call but the first. Mirrors | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#Joiner) |
| `LoopContext` | type | LoopContext is the value the Jinja `loop` variable holds inside a `&#123;% for %}` | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#LoopContext) |
| `Namespace` | type | Namespace is a mutable container of named values, equivalent to Jinja2's | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#Namespace) |
| `NamespaceEntry` | type | NamespaceEntry is a (name, value) pair returned by [Namespace.Items]. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NamespaceEntry) |
| `NewBase` | func | NewBase returns a ModeBase Undefined. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewBase) |
| `NewChainable` | func | NewChainable returns a ModeChainable Undefined. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewChainable) |
| `NewContext` | func | NewContext constructs a Context with the supplied parent globals and a | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewContext) |
| `NewCycler` | func | NewCycler builds a Cycler over items. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewCycler) |
| `NewDebug` | func | NewDebug returns a ModeDebug Undefined. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewDebug) |
| `NewEvalContext` | func | NewEvalContext constructs an EvalContext for the given (env, template | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewEvalContext) |
| `NewJoiner` | func | NewJoiner returns a Joiner that emits sep between items (default ", "). | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewJoiner) |
| `NewLoopContext` | func | NewLoopContext constructs a LoopContext over items. depth is 0 for top- | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewLoopContext) |
| `NewNamespace` | func | NewNamespace constructs a Namespace seeded from the given map. The map | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewNamespace) |
| `NewOrderedDict` | func | NewOrderedDict returns an empty OrderedDict ready for Set. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewOrderedDict) |
| `NewOrderedDictFromPairs` | func | NewOrderedDictFromPairs builds an OrderedDict from a sequence of | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewOrderedDictFromPairs) |
| `NewStrict` | func | NewStrict returns a ModeStrict Undefined. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#NewStrict) |
| `OrderedDict` | type | OrderedDict is an insertion-ordered map[any]any — the runtime | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#OrderedDict) |
| `PassArg` | type | PassArg names the categories of "first-arg injection" that filters and | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#PassArg) |
| `Tuple` | type | Tuple is a fixed-size ordered sequence — gojinja's runtime | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#Tuple) |
| `Undefined` | type | Undefined is gojinja's representation of "no value at this name". It | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#Undefined) |
| `UndefinedFactory` | type | UndefinedFactory is the constructor signature an Environment registers | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#UndefinedFactory) |
| `UndefinedMode` | type | UndefinedMode selects the behavioral variant of an Undefined value. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#UndefinedMode) |
| `Undefiner` | type | Undefiner is implemented by every Undefined value. Use [IsUndefined] to | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/runtime#Undefiner) |

## `github.com/jryberg/gojinja/pkg/sandbox`

| Symbol | Kind | Synopsis | Reference |
|---|---|---|---|
| `IsMutatingMethod` | func | IsMutatingMethod returns true if attr is a known mutating method name | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/sandbox#IsMutatingMethod) |
| `IsSafeAttribute` | func | IsSafeAttribute reports whether `attr` is allowed on `obj`. | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/sandbox#IsSafeAttribute) |
| `IsSafeCallable` | func | IsSafeCallable reports whether the given value may be invoked from a | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/sandbox#IsSafeCallable) |
| `MutatingMethods` | var | MutatingMethods enumerates method names that should be blocked under | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/sandbox#MutatingMethods) |
| `UnsafeFunc` | type | UnsafeFunc is a wrapper that marks a callable unsafe; the env's Call | [pkg.go.dev](https://pkg.go.dev/github.com/jryberg/gojinja/pkg/sandbox#UnsafeFunc) |
