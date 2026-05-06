// Manifest of the 10 upstream projects whose templates we run through the
// gojinja-vs-Python-Jinja2 parity harness. Each entry is pinned to a
// specific commit; running the fetcher with a different pin yields a
// different cache. The manifest is the canonical record — `SOURCES.md`
// next to this file is the human-readable mirror.
package main

// project describes a single upstream we sample templates from.
type project struct {
	// Name is the local cache directory name, e.g. "jinja", "flask".
	Name string
	// Repo is the GitHub "owner/name" used to build URLs.
	Repo string
	// Commit is the pinned SHA. Tarballs are fetched at this exact ref.
	Commit string
	// License lists upstream paths whose contents are vendored next to
	// the templates as proof-of-license. Multiple paths cover repos
	// that ship LICENSE + NOTICE separately.
	License []string
	// Includes is a set of upstream-relative path prefixes. A file is
	// kept iff its tarball-relative path starts with any prefix AND its
	// suffix is in Suffixes.
	Includes []string
	// Suffixes restricts the file extensions copied.
	Suffixes []string
	// MaxFiles caps the number of template files copied (0 = no cap).
	// Used for very large repos (netbox) so the cache stays manageable.
	MaxFiles int
	// LoaderRoot, when non-empty, is the cache-relative directory used
	// as the FileSystem loader root for extends/include resolution
	// during render-parity. Empty = no loader (parse-only).
	LoaderRoot string
}

// projects is the manifest, ordered to match SOURCES.md.
var projects = []project{
	{
		Name:    "jinja",
		Repo:    "pallets/jinja",
		Commit:  "5ef70112a1ff19c05324ff889dd30405b1002044",
		License: []string{"LICENSE.txt"},
		Includes: []string{
			"tests/res/templates/",
		},
		Suffixes:   []string{".html", ".txt", ".j2", ".jinja"},
		LoaderRoot: "tests/res/templates",
	},
	{
		Name:    "flask",
		Repo:    "pallets/flask",
		Commit:  "7374c85ddefc3f4b177a698ab9f0cbb6a5c0b392",
		License: []string{"LICENSE.txt"},
		Includes: []string{
			"examples/tutorial/flaskr/templates/",
		},
		Suffixes:   []string{".html", ".j2", ".jinja"},
		LoaderRoot: "examples/tutorial/flaskr/templates",
	},
	{
		Name:    "sphinx",
		Repo:    "sphinx-doc/sphinx",
		Commit:  "cc7c6f435ad37bb12264f8118c8461b230e6830c",
		License: []string{"LICENSE.rst"},
		Includes: []string{
			"sphinx/themes/",
		},
		Suffixes:   []string{".html"},
		LoaderRoot: "sphinx/themes",
	},
	{
		Name:    "mkdocs",
		Repo:    "mkdocs/mkdocs",
		Commit:  "2862536793b3c67d9d83c33e0dd6d50a791928f8",
		License: []string{"LICENSE"},
		Includes: []string{
			"mkdocs/themes/mkdocs/",
			"mkdocs/themes/readthedocs/",
		},
		Suffixes:   []string{".html"},
		LoaderRoot: "mkdocs/themes",
	},
	{
		Name:    "mkdocs-material",
		Repo:    "squidfunk/mkdocs-material",
		Commit:  "8d01326cd2e8d8030d39e6d69790bd01fa3b7e46",
		License: []string{"LICENSE"},
		Includes: []string{
			"material/templates/",
		},
		Suffixes:   []string{".html"},
		LoaderRoot: "material/templates",
	},
	{
		Name:    "nbconvert",
		Repo:    "jupyter/nbconvert",
		Commit:  "78ed30837a607deab7cf0a12dca072bf3f63417a",
		License: []string{"LICENSE"},
		Includes: []string{
			"share/templates/",
		},
		Suffixes:   []string{".j2", ".tpl"},
		LoaderRoot: "share/templates",
	},
	{
		Name:    "cookiecutter",
		Repo:    "cookiecutter/cookiecutter",
		Commit:  "c88fbe921c97c58b65f1883ba90a0ab53cc91b34",
		License: []string{"LICENSE"},
		Includes: []string{
			"tests/fake-repo-tmpl/",
			"tests/fake-repo-pre/",
			"tests/fake-repo-dict/",
			"tests/files/",
			"tests/test-pyhooks/",
		},
		Suffixes: []string{".rst", ".md", ".txt", ".html", ".j2"},
	},
	{
		Name:    "airflow",
		Repo:    "apache/airflow",
		Commit:  "4d8eae3f28d981bc7afca0143e3d8d24e71d89b7",
		License: []string{"LICENSE", "NOTICE"},
		Includes: []string{
			"providers/fab/src/airflow/providers/fab/www/templates/",
			"airflow-core/docs/templates/",
			"devel-common/src/sphinx_exts/pagefind_search/templates/",
		},
		Suffixes: []string{".html"},
	},
	{
		Name:    "dbt-adapters",
		Repo:    "dbt-labs/dbt-adapters",
		Commit:  "0f260a278fa9a3e6e2750d6e608ebcb2bcb0cde6",
		License: []string{"License.md", "LICENSE", "LICENSE.md"},
		Includes: []string{
			"dbt-adapters/src/dbt/include/global_project/macros/",
		},
		Suffixes:   []string{".sql"},
		LoaderRoot: "dbt-adapters/src/dbt/include/global_project/macros",
	},
	{
		Name:    "netbox",
		Repo:    "netbox-community/netbox",
		Commit:  "30f9d3ed604e2a227a0835ce200bd5958707b1c6",
		License: []string{"LICENSE.txt"},
		Includes: []string{
			"netbox/templates/",
		},
		Suffixes:   []string{".html"},
		MaxFiles:   100,
		LoaderRoot: "netbox/templates",
	},
}
