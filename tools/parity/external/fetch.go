// Tarball-based fetcher for the external parity corpus. We pull each
// pinned upstream as a single GitHub tarball, stream it through gzip+tar,
// and write only the files that match the per-project include/suffix rules
// into tools/parity/external/cache/<project>/. Anything else is discarded
// in flight, so the on-disk footprint stays small even for huge repos.
package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// fetchAll fetches every project in the manifest into cacheDir, replacing
// any existing per-project subdirectory. Returns the first error.
func fetchAll(cacheDir string) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	for _, p := range projects {
		fmt.Printf("==> %s @ %s\n", p.Repo, shortSHA(p.Commit))
		if err := fetchOne(cacheDir, p); err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
	}
	return nil
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// fetchOne downloads a single project tarball and extracts the matching
// files. The tarball stream is consumed once — we never write the full
// archive to disk.
func fetchOne(cacheDir string, p project) error {
	dst := filepath.Join(cacheDir, p.Name)
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/tarball/%s", p.Repo, p.Commit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gojinja-parity-fetcher")
	if tok := os.Getenv("GH_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	} else if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	manifest := map[string]string{} // local rel path → upstream rel path
	licenseHits := map[string]bool{}
	templateCount := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		// Strip the leading "<owner>-<name>-<sha>/" component.
		rel := stripTopDir(hdr.Name)
		if rel == "" {
			continue
		}

		// License files: write under __LICENSE__/<basename>
		if isLicensePath(rel, p.License) {
			licenseHits[rel] = true
			licDir := filepath.Join(dst, "__LICENSE__")
			if err := os.MkdirAll(licDir, 0o755); err != nil {
				return err
			}
			out := filepath.Join(licDir, filepath.Base(rel))
			if err := writeTarFile(tr, out, hdr.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}

		// Template files: keep iff it matches an include prefix AND a suffix.
		if !matchesProject(rel, p) {
			continue
		}
		if p.MaxFiles > 0 && templateCount >= p.MaxFiles {
			// Drain — but discard the rest. Sorted-by-tarball-order is
			// deterministic for a given commit.
			io.Copy(io.Discard, tr)
			continue
		}
		out := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := writeTarFile(tr, out, hdr.FileInfo().Mode()); err != nil {
			return err
		}
		manifest[rel] = rel
		templateCount++
	}

	if templateCount == 0 {
		return fmt.Errorf("no template files matched (includes=%v suffixes=%v)", p.Includes, p.Suffixes)
	}
	if len(licenseHits) == 0 {
		return fmt.Errorf("license file not found in tarball (looked for %v)", p.License)
	}

	// Write the per-cache manifest (path → upstream path) for traceability.
	mPath := filepath.Join(dst, "__MANIFEST__.json")
	mfData, _ := json.MarshalIndent(struct {
		Project string            `json:"project"`
		Repo    string            `json:"repo"`
		Commit  string            `json:"commit"`
		Files   map[string]string `json:"files"`
		Count   int               `json:"count"`
	}{p.Name, p.Repo, p.Commit, manifest, templateCount}, "", "  ")
	if err := os.WriteFile(mPath, mfData, 0o644); err != nil {
		return err
	}

	keys := sortedKeys(manifest)
	tail := keys
	if len(tail) > 3 {
		tail = tail[:3]
	}
	fmt.Printf("    %d files; sample: %s\n", templateCount, strings.Join(tail, ", "))
	return nil
}

func writeTarFile(tr io.Reader, path string, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	// Cap individual file size at 16 MiB — anything larger isn't a template.
	_, copyErr := io.Copy(f, io.LimitReader(tr, 16<<20))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func stripTopDir(p string) string {
	// Tarball entries look like "pallets-jinja-5ef7011/path/to/file".
	// We drop the first component.
	_, rest, ok := strings.Cut(p, "/")
	if !ok {
		return ""
	}
	return rest
}

func matchesProject(rel string, p project) bool {
	hitInclude := false
	for _, inc := range p.Includes {
		if strings.HasPrefix(rel, inc) {
			hitInclude = true
			break
		}
	}
	if !hitInclude {
		return false
	}
	for _, suf := range p.Suffixes {
		if strings.HasSuffix(rel, suf) {
			return true
		}
	}
	return false
}

func isLicensePath(rel string, candidates []string) bool {
	return slices.Contains(candidates, rel)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
