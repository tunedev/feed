package crawl_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tunedev/feed/internal/crawl"
	"github.com/tunedev/feed/internal/normalize"
	"github.com/tunedev/feed/internal/store"
)

var t0 = time.Date(2026, 9, 24, 6, 0, 0, 0, time.UTC)

func clock() time.Time { return t0 }

type fake struct {
	name     string
	rendered bool
	postings []normalize.Posting
	err      error
}

func (f fake) Name() string   { return f.name }
func (f fake) Rendered() bool { return f.rendered }
func (f fake) Fetch(context.Context) ([]normalize.Posting, error) {
	return f.postings, f.err
}

func posting(name, ext string) normalize.Posting {
	source, board, _ := strings.Cut(name, "/")
	p := normalize.New(source, board, ext)
	p.Title = "Role " + ext
	p.URL = "https://example.com/" + ext
	return p
}

func run(t *testing.T, root string, fs ...crawl.Fetcher) crawl.Manifest {
	t.Helper()
	m, err := crawl.Run(context.Background(), root, fs, clock, 90*24*time.Hour)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return m
}

func decode[T any](t *testing.T, path string) T {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return v
}

func TestRunRecordsEachFetcherAndKeepsGoingPastAFailure(t *testing.T) {
	root := t.TempDir()
	kept := posting("lever/down", "keep")
	kept.FetchedAt = normalize.Stamp(t0.Add(-time.Hour))
	if err := store.WriteJSON(root, "postings/lever/down/keep.json", kept); err != nil {
		t.Fatal(err)
	}

	m := run(t, root,
		fake{name: "greenhouse/up", postings: []normalize.Posting{posting("greenhouse/up", "1")}},
		fake{name: "lever/down", err: errors.New("returned 503")},
		fake{name: "rendered/shell", rendered: true, err: fmt.Errorf("no browser: %w", crawl.ErrNeedsRendering)},
	)

	if m.AllFailed() {
		t.Error("AllFailed with one fetcher succeeding")
	}
	want := map[string][2]any{
		"greenhouse/up":  {"static", 1},
		"lever/down":     {"static", 0},
		"rendered/shell": {"needs_rendering", 0},
	}
	for name, w := range want {
		s := decode[crawl.Status](t, filepath.Join(root, "status", name+".json"))
		if s.RenderStatus != w[0] || s.PostingCount != w[1] || s.CheckedAt != "2026-09-24T06:00:00Z" {
			t.Errorf("status %s = %+v, want render %v count %v", name, s, w[0], w[1])
		}
	}
	if s := decode[crawl.Status](t, filepath.Join(root, "status", "lever", "down.json")); !strings.Contains(s.Error, "503") {
		t.Errorf("failed board's status error = %q", s.Error)
	}
	if p := decode[normalize.Posting](t, filepath.Join(root, "postings", "lever", "down", "keep.json")); p.Status != "open" {
		t.Error("a board that failed to fetch had its posting marked removed")
	}
	manifest := decode[crawl.Manifest](t, filepath.Join(root, "runs", "20260924T060000Z.json"))
	if len(manifest.Fetchers) != 3 || manifest.Changes.Written != 1 {
		t.Errorf("manifest = %+v", manifest)
	}
}

func TestAllFailedOnlyWhenEveryFetcherFailed(t *testing.T) {
	down := fake{name: "lever/a", err: errors.New("boom")}
	alsoDown := fake{name: "lever/b", err: errors.New("boom")}
	shell := fake{name: "rendered/c", rendered: true, err: crawl.ErrNeedsRendering}
	up := fake{name: "greenhouse/up", postings: []normalize.Posting{posting("greenhouse/up", "1")}}

	root := t.TempDir()
	if m := run(t, root, down, alsoDown); !m.AllFailed() {
		t.Error("two failures did not report AllFailed")
	}
	if _, err := os.Stat(filepath.Join(root, "runs", "20260924T060000Z.json")); err != nil {
		t.Errorf("no manifest after every fetcher failed: %v", err)
	}
	if m := run(t, t.TempDir(), down, shell); !m.AllFailed() {
		t.Error("a run where nothing succeeded, only failures and a board needing rendering, did not report AllFailed")
	}
	if m := run(t, t.TempDir(), up, shell, down); m.AllFailed() {
		t.Error("one fetcher succeeding alongside a failure and a needs-rendering board was counted as AllFailed")
	}
}

func TestRunRejectsPostingsThatCannotBeWritten(t *testing.T) {
	root := t.TempDir()
	good := posting("greenhouse/up", "1")
	escape := posting("greenhouse/up", "..")
	elsewhere := posting("greenhouse/other", "2")
	m := run(t, root, fake{name: "greenhouse/up", postings: []normalize.Posting{good, escape, elsewhere, good}})
	if f := m.Fetchers[0]; f.PostingCount != 1 || f.Rejected != 3 {
		t.Errorf("fetcher run = %+v, want 1 kept and 3 rejected", f)
	}
	if _, err := os.Stat(filepath.Join(root, "postings", "greenhouse", "other")); err == nil {
		t.Error("a posting under another board's name was written")
	}
}

func TestFetchOneFailsWhenEveryPostingIsRejected(t *testing.T) {
	root := t.TempDir()
	kept := posting("greenhouse/acme", "keep")
	kept.FetchedAt = normalize.Stamp(t0.Add(-time.Hour))
	if err := store.WriteJSON(root, "postings/greenhouse/acme/keep.json", kept); err != nil {
		t.Fatal(err)
	}

	shapeChanged := posting("greenhouse/other", "1")
	m := run(t, root, fake{name: "greenhouse/acme", postings: []normalize.Posting{shapeChanged}})

	if f := m.Fetchers[0]; f.Error == "" {
		t.Errorf("fetcher run = %+v, want an error when every posting was rejected", f)
	}
	if p := decode[normalize.Posting](t, filepath.Join(root, "postings", "greenhouse", "acme", "keep.json")); p.Status != "open" {
		t.Error("a board whose fetch was entirely rejected had its posting marked removed")
	}
}

func TestARenderedSuccessIsRendered(t *testing.T) {
	root := t.TempDir()
	run(t, root, fake{name: "rendered/acme", rendered: true})
	if s := decode[crawl.Status](t, filepath.Join(root, "status", "rendered", "acme.json")); s.RenderStatus != "rendered" {
		t.Errorf("render_status = %q, want rendered", s.RenderStatus)
	}
}
