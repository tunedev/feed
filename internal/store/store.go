// Package store writes the feed's current-state tree: one file per posting,
// rewritten only when the posting changed, marked removed rather than
// deleted when it vanishes, and deleted only after it has been removed for
// longer than the retention window.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tunedev/feed/internal/normalize"
)

// Observation is what one successful fetch of one board saw. Name is the
// board's "<source>/<board>" address.
type Observation struct {
	Name     string
	Postings []normalize.Posting
}

// Changes counts what Apply did to the tree.
type Changes struct {
	Written  int `json:"written"`
	Removed  int `json:"removed"`
	Reopened int `json:"reopened"`
	Deleted  int `json:"deleted"`
}

// Apply records each observation under root/postings at time now, then
// deletes removed postings older than retention. Only an observed board is
// compared against what it held before, so a board absent from observed
// keeps every posting it has.
func Apply(root string, now time.Time, observed []Observation, retention time.Duration) (Changes, error) {
	var c Changes
	for _, o := range observed {
		if err := c.observe(root, now, o); err != nil {
			return c, err
		}
	}
	deleted, err := expire(root, now, retention)
	c.Deleted = deleted
	return c, err
}

// observe writes each observed posting that is new or changed, and marks
// each previously open posting the board no longer lists as removed.
func (c *Changes) observe(root string, now time.Time, o Observation) error {
	known, err := readBoard(root, o.Name)
	if err != nil {
		return err
	}
	stamp := normalize.Stamp(now)
	seen := make(map[string]bool, len(o.Postings))
	for _, p := range o.Postings {
		seen[p.ID] = true
		prev, had := known[p.ID]
		p.FetchedAt, p.Status, p.RemovedAt = stamp, normalize.StatusOpen, ""
		if had && sameExceptFetchedAt(prev, p) {
			continue
		}
		if had && prev.Status == normalize.StatusRemoved {
			c.Reopened++
		}
		if err := WriteJSON(root, postingPath(p.ID), p); err != nil {
			return err
		}
		c.Written++
	}
	for id, prev := range known {
		if seen[id] || prev.Status == normalize.StatusRemoved {
			continue
		}
		prev.Status, prev.RemovedAt = normalize.StatusRemoved, stamp
		if err := WriteJSON(root, postingPath(id), prev); err != nil {
			return err
		}
		c.Removed++
	}
	return nil
}

func sameExceptFetchedAt(a, b normalize.Posting) bool {
	a.FetchedAt, b.FetchedAt = "", ""
	return a == b
}

func postingPath(id string) string { return "postings/" + id + ".json" }

// readBoard loads every posting file under postings/<name>, keyed by id. A
// board with no directory yet holds nothing.
func readBoard(root, name string) (map[string]normalize.Posting, error) {
	dir := filepath.Join(root, "postings", filepath.FromSlash(name))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]normalize.Posting{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: read %s: %w", dir, err)
	}
	out := make(map[string]normalize.Posting, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p, err := readPosting(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out[p.ID] = p
	}
	return out, nil
}

func readPosting(path string) (normalize.Posting, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return normalize.Posting{}, fmt.Errorf("store: read %s: %w", path, err)
	}
	var p normalize.Posting
	if err := json.Unmarshal(raw, &p); err != nil {
		return normalize.Posting{}, fmt.Errorf("store: decode %s: %w", path, err)
	}
	return p, nil
}

// expire deletes every removed posting whose removed_at is more than
// retention before now, and returns how many it deleted.
func expire(root string, now time.Time, retention time.Duration) (int, error) {
	dir := filepath.Join(root, "postings")
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	deleted := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		p, err := readPosting(path)
		if err != nil {
			return err
		}
		if p.Status != normalize.StatusRemoved {
			return nil
		}
		removed, err := time.Parse(time.RFC3339, p.RemovedAt)
		if err != nil {
			return fmt.Errorf("store: %s is removed with removed_at %q: %w", path, p.RemovedAt, err)
		}
		if now.Sub(removed) <= retention {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("store: delete %s: %w", path, err)
		}
		deleted++
		return nil
	})
	return deleted, err
}

// WriteJSON writes v, encoded as every feed document is, to rel under root,
// creating directories as needed.
func WriteJSON(root, rel string, v any) error {
	body, err := normalize.Encode(v)
	if err != nil {
		return err
	}
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("store: create directories for %s: %w", rel, err)
	}
	if err := os.WriteFile(full, body, 0o644); err != nil {
		return fmt.Errorf("store: write %s: %w", rel, err)
	}
	return nil
}
