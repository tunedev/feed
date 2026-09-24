package crawl_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tunedev/feed/internal/crawl"
	"github.com/tunedev/feed/internal/normalize"
)

// topLevel is everything the feed repository may hold at its root.
var topLevel = map[string]bool{
	".git": true, ".github": true, ".gitignore": true, "README.md": true, "CHANGELOG.md": true,
	"boards.yaml": true, "schema": true, "docs": true, "cmd": true, "internal": true,
	"go.mod": true, "go.sum": true, "postings": true, "status": true, "runs": true,
}

// checkTree reports every way root holds something other than the feed's
// own code and its three document shapes. Each document must decode with no
// unknown field, so a field carrying anything else — a CV, a decision, a
// preference — fails it.
func checkTree(root string) []error {
	var problems []error
	entries, err := os.ReadDir(root)
	if err != nil {
		return []error{err}
	}
	for _, e := range entries {
		if !topLevel[e.Name()] {
			problems = append(problems, fmt.Errorf("unexpected top-level entry %q", e.Name()))
		}
	}
	problems = append(problems, checkDocs(root, "postings", func(rel string, dec *json.Decoder) error {
		var p normalize.Posting
		if err := dec.Decode(&p); err != nil {
			return err
		}
		if err := p.Validate(); err != nil {
			return err
		}
		if want := "postings/" + p.ID + ".json"; rel != want {
			return fmt.Errorf("id %q does not match its path", p.ID)
		}
		return nil
	})...)
	problems = append(problems, checkDocs(root, "status", strict[crawl.Status])...)
	problems = append(problems, checkDocs(root, "runs", strict[crawl.Manifest])...)
	return problems
}

func strict[T any](_ string, dec *json.Decoder) error {
	var v T
	return dec.Decode(&v)
}

func checkDocs(root, dir string, check func(rel string, dec *json.Decoder) error) []error {
	base := filepath.Join(root, dir)
	if _, err := os.Stat(base); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	var problems []error
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, ".json") {
			problems = append(problems, fmt.Errorf("%s: only JSON documents belong under %s/", rel, dir))
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := check(rel, dec); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", rel, err))
		}
		return nil
	})
	if err != nil {
		problems = append(problems, err)
	}
	return problems
}

// The feed holds normalised postings and the crawl's own records, and
// nothing else. crawl.yml runs this against the tree it is about to commit.
func TestTreeHoldsOnlyFeedDocuments(t *testing.T) {
	for _, p := range checkTree(filepath.Join("..", "..")) {
		t.Error(p)
	}
}

func TestCheckTreeCatchesWhatDoesNotBelong(t *testing.T) {
	p := normalize.New("lever", "acme", "a")
	p.Title, p.URL = "Engineer", "https://example.com/a"
	good, err := normalize.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	foreign := bytes.Replace(good, []byte(`"title"`), []byte(`"cv": "private", "title"`), 1)

	cases := map[string]map[string][]byte{
		"foreign field":       {"postings/lever/acme/a.json": foreign},
		"id not at its path":  {"postings/lever/acme/b.json": good},
		"non-JSON under runs": {"runs/notes.txt": []byte("hello")},
		"unexpected root":     {"profile.json": []byte("{}")},
	}
	for name, files := range cases {
		root := t.TempDir()
		for rel, body := range files {
			full := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if len(checkTree(root)) == 0 {
			t.Errorf("%s: checkTree found nothing wrong", name)
		}
	}
}
