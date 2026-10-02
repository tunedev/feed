package store_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tunedev/feed/internal/normalize"
	"github.com/tunedev/feed/internal/store"
)

var (
	t0        = time.Date(2026, 9, 24, 6, 0, 0, 0, time.UTC)
	t1        = t0.Add(6 * time.Hour)
	t2        = t1.Add(6 * time.Hour)
	retention = 90 * 24 * time.Hour
)

func posting(ext, title string) normalize.Posting {
	p := normalize.New("lever", "acme", ext)
	p.Company = "Acme"
	p.Title = title
	p.URL = "https://example.com/" + ext
	return p
}

func observe(ps ...normalize.Posting) []store.Observation {
	return []store.Observation{{Name: "lever/acme", Postings: ps}}
}

func apply(t *testing.T, root string, now time.Time, obs []store.Observation) store.Changes {
	t.Helper()
	c, err := store.Apply(root, now, obs, retention)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return c
}

func raw(t *testing.T, root, ext string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "postings", "lever", "acme", ext+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func read(t *testing.T, root, ext string) normalize.Posting {
	t.Helper()
	var p normalize.Posting
	if err := json.Unmarshal(raw(t, root, ext), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestApplyWritesNewPostingsStampedWithTheCrawlTime(t *testing.T) {
	root := t.TempDir()
	c := apply(t, root, t0, observe(posting("a", "Engineer")))
	if c.Written != 1 {
		t.Errorf("Written = %d, want 1", c.Written)
	}
	p := read(t, root, "a")
	if p.FetchedAt != "2026-09-24T06:00:00Z" || p.Status != "open" || p.Title != "Engineer" {
		t.Errorf("written posting = %+v", p)
	}
}

func TestApplyLeavesAnUnchangedPostingByteForByte(t *testing.T) {
	root := t.TempDir()
	apply(t, root, t0, observe(posting("a", "Engineer")))
	before := raw(t, root, "a")
	c := apply(t, root, t1, observe(posting("a", "Engineer")))
	if c.Written != 0 {
		t.Errorf("Written = %d, want 0: nothing changed", c.Written)
	}
	if !bytes.Equal(before, raw(t, root, "a")) {
		t.Error("an unchanged posting was rewritten; its git history would gain a commit every crawl")
	}
}

func TestApplyRewritesAChangedPosting(t *testing.T) {
	root := t.TempDir()
	apply(t, root, t0, observe(posting("a", "Engineer")))
	c := apply(t, root, t1, observe(posting("a", "Senior Engineer")))
	if c.Written != 1 {
		t.Errorf("Written = %d, want 1", c.Written)
	}
	if p := read(t, root, "a"); p.Title != "Senior Engineer" || p.FetchedAt != normalize.Stamp(t1) {
		t.Errorf("rewritten posting = %+v", p)
	}
}

func TestApplyMarksAnAbsentPostingRemovedOnceAndKeepsItsFields(t *testing.T) {
	root := t.TempDir()
	apply(t, root, t0, observe(posting("a", "Engineer"), posting("b", "Designer")))
	c := apply(t, root, t1, observe(posting("a", "Engineer")))
	if c.Removed != 1 {
		t.Errorf("Removed = %d, want 1", c.Removed)
	}
	b := read(t, root, "b")
	if b.Status != "removed" || b.RemovedAt != normalize.Stamp(t1) || b.Title != "Designer" || b.FetchedAt != normalize.Stamp(t0) {
		t.Errorf("removed posting = %+v", b)
	}
	if c := apply(t, root, t2, observe(posting("a", "Engineer"))); c.Removed != 0 {
		t.Errorf("second absence: Removed = %d, want 0", c.Removed)
	}
	if read(t, root, "b").RemovedAt != normalize.Stamp(t1) {
		t.Error("removed_at moved on a later crawl; it is set once")
	}
}

func TestApplyReopensAPostingThatReturns(t *testing.T) {
	root := t.TempDir()
	apply(t, root, t0, observe(posting("a", "Engineer"), posting("b", "Designer")))
	apply(t, root, t1, observe(posting("a", "Engineer")))
	c := apply(t, root, t2, observe(posting("a", "Engineer"), posting("b", "Designer")))
	if c.Reopened != 1 {
		t.Errorf("Reopened = %d, want 1", c.Reopened)
	}
	if b := read(t, root, "b"); b.Status != "open" || b.RemovedAt != "" {
		t.Errorf("reopened posting = %+v", b)
	}
	if strings.Contains(string(raw(t, root, "b")), "removed_at") {
		t.Error("a reopened posting still carries removed_at")
	}
}

func TestApplyLeavesAnUnobservedBoardAlone(t *testing.T) {
	root := t.TempDir()
	apply(t, root, t0, observe(posting("a", "Engineer")))
	apply(t, root, t1, nil)
	if p := read(t, root, "a"); p.Status != "open" {
		t.Errorf("a board that was not observed lost its posting: %+v", p)
	}
}

func TestApplyDeletesRemovedPostingsPastRetentionOnly(t *testing.T) {
	root := t.TempDir()
	seed := func(ext, status string, removedAgo time.Duration) {
		p := posting(ext, "Role")
		p.Status = status
		p.FetchedAt = normalize.Stamp(t0.Add(-200 * 24 * time.Hour))
		if status == "removed" {
			p.RemovedAt = normalize.Stamp(t0.Add(-removedAgo))
		}
		if err := store.WriteJSON(root, "postings/lever/acme/"+ext+".json", p); err != nil {
			t.Fatal(err)
		}
	}
	seed("old", "removed", 91*24*time.Hour)
	seed("recent", "removed", 89*24*time.Hour)
	seed("open", "open", 0)

	c := apply(t, root, t0, nil)
	if c.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1", c.Deleted)
	}
	dir := filepath.Join(root, "postings", "lever", "acme")
	for ext, want := range map[string]bool{"old": false, "recent": true, "open": true} {
		_, err := os.Stat(filepath.Join(dir, ext+".json"))
		if exists := err == nil; exists != want {
			t.Errorf("%s exists = %v, want %v", ext, exists, want)
		}
	}
}

func TestApplyFailsOnACorruptPosting(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "postings", "lever", "acme")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Apply(root, t0, observe(posting("a", "Engineer")), retention); err == nil {
		t.Error("Apply succeeded over a corrupt file; it must fail rather than guess")
	}
}
