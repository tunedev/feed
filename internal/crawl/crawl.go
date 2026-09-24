// Package crawl runs every fetcher once, applies what each observed to the
// posting tree, and writes a status file per board and a manifest per run.
// One fetcher failing never stops the others.
package crawl

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/tunedev/feed/internal/normalize"
	"github.com/tunedev/feed/internal/store"
)

// ErrNeedsRendering is how a fetcher says its page holds no roles until a
// browser renders it. It is a signal, not a failure.
var ErrNeedsRendering = errors.New("needs rendering")

// Fetcher pulls one board's current postings. Name is the board's
// "<source>/<board>" address; Rendered says whether Fetch drives a browser.
// Fetch does no writing.
type Fetcher interface {
	Name() string
	Rendered() bool
	Fetch(ctx context.Context) ([]normalize.Posting, error)
}

// Status is status/<source>/<board>.json: the last crawl's outcome for one
// board, written whether or not the fetch succeeded.
type Status struct {
	Source       string `json:"source"`
	Board        string `json:"board"`
	RenderStatus string `json:"render_status"`
	CheckedAt    string `json:"checked_at"`
	PostingCount int    `json:"posting_count"`
	Error        string `json:"error,omitempty"`
}

// FetcherRun is one fetcher's entry in a run's manifest.
type FetcherRun struct {
	Name         string `json:"name"`
	RenderStatus string `json:"render_status"`
	PostingCount int    `json:"posting_count"`
	Rejected     int    `json:"rejected"`
	Error        string `json:"error,omitempty"`
}

// Manifest is runs/<timestamp>.json: metadata about one crawl, never a copy
// of its data.
type Manifest struct {
	StartedAt  string        `json:"started_at"`
	FinishedAt string        `json:"finished_at"`
	Fetchers   []FetcherRun  `json:"fetchers"`
	Changes    store.Changes `json:"changes"`
}

// AllFailed reports whether every fetcher failed. A board that needs
// rendering has not failed: it said what it needs.
func (m Manifest) AllFailed() bool {
	for _, f := range m.Fetchers {
		if f.Error == "" || f.RenderStatus == normalize.RenderNeedsRendering {
			return false
		}
	}
	return len(m.Fetchers) > 0
}

// Run fetches every board in order, writes each board's status, applies the
// successful observations to the tree, and writes the manifest. It errors
// only when writing fails; a failing fetcher is recorded, not returned.
func Run(ctx context.Context, root string, fetchers []Fetcher, now func() time.Time, retention time.Duration) (Manifest, error) {
	started := now()
	m := Manifest{StartedAt: normalize.Stamp(started)}
	var observed []store.Observation
	for _, f := range fetchers {
		run, obs := fetchOne(ctx, f)
		m.Fetchers = append(m.Fetchers, run)
		if err := writeStatus(root, run, now()); err != nil {
			return m, err
		}
		if obs != nil {
			observed = append(observed, *obs)
		}
	}
	changes, err := store.Apply(root, started, observed, retention)
	if err != nil {
		return m, err
	}
	m.Changes = changes
	m.FinishedAt = normalize.Stamp(now())
	return m, store.WriteJSON(root, "runs/"+started.UTC().Format("20060102T150405Z")+".json", m)
}

// fetchOne runs one fetcher and returns its manifest entry, plus the
// observation to apply when it succeeded.
func fetchOne(ctx context.Context, f Fetcher) (FetcherRun, *store.Observation) {
	postings, err := f.Fetch(ctx)
	run := FetcherRun{Name: f.Name(), RenderStatus: renderStatus(f, err)}
	if err != nil {
		run.Error = err.Error()
		return run, nil
	}
	valid, rejected := validate(f.Name(), postings)
	run.PostingCount, run.Rejected = len(valid), rejected
	return run, &store.Observation{Name: f.Name(), Postings: valid}
}

func renderStatus(f Fetcher, err error) string {
	switch {
	case errors.Is(err, ErrNeedsRendering):
		return normalize.RenderNeedsRendering
	case !f.Rendered():
		return normalize.RenderStatic
	case err != nil:
		return normalize.RenderError
	default:
		return normalize.RenderRendered
	}
}

// validate keeps the postings that can be written under name, dropping any
// that fail Posting.Validate, sit under another board, or repeat an id.
func validate(name string, postings []normalize.Posting) ([]normalize.Posting, int) {
	seen := make(map[string]bool, len(postings))
	var valid []normalize.Posting
	for _, p := range postings {
		if p.Validate() != nil || !strings.HasPrefix(p.ID, name+"/") || seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		valid = append(valid, p)
	}
	return valid, len(postings) - len(valid)
}

func writeStatus(root string, run FetcherRun, checked time.Time) error {
	source, board, _ := strings.Cut(run.Name, "/")
	return store.WriteJSON(root, "status/"+run.Name+".json", Status{
		Source:       source,
		Board:        board,
		RenderStatus: run.RenderStatus,
		CheckedAt:    normalize.Stamp(checked),
		PostingCount: run.PostingCount,
		Error:        run.Error,
	})
}
