package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tunedev/feed/internal/crawl"
)

// The dry run is the whole pipeline — boards.yaml, every factory, every
// fetcher's request and mapping, the tree, the manifest — against recorded
// responses, with no network.
func TestDryRunCrawlsEveryBoardFromFixtures(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-dry-run", "-boards", filepath.Join("..", "..", "boards.yaml")}, &out); err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	first, _, _ := strings.Cut(out.String(), "\n")
	root := strings.TrimPrefix(first, "dry run into ")
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	runs, err := filepath.Glob(filepath.Join(root, "runs", "*.json"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("manifests = %v, %v; want exactly one", runs, err)
	}
	b, err := os.ReadFile(runs[0])
	if err != nil {
		t.Fatal(err)
	}
	var m crawl.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, f := range m.Fetchers {
		if f.RenderStatus == "needs_rendering" {
			continue
		}
		if f.Error != "" || f.PostingCount == 0 {
			t.Errorf("%s: %d postings, error %q", f.Name, f.PostingCount, f.Error)
		}
	}
}
