// Package remoteok fetches RemoteOK's single global feed.
package remoteok

import (
	"context"
	"fmt"
	"time"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/fetch/httpjson"
	"github.com/tunedev/feed/internal/normalize"
)

const Base = "https://remoteok.com"

type Fetcher struct {
	board  boards.Board
	client *httpjson.Client
	base   string
}

// New serves RemoteOK's one global feed; the board's slug only names it in
// the feed's paths. Its edge refuses requests without a descriptive
// User-Agent, which the httpjson client always sends.
func New(b boards.Board, c *httpjson.Client, base string) *Fetcher {
	return &Fetcher{board: b, client: c, base: base}
}

func (f *Fetcher) Name() string   { return f.board.Name() }
func (f *Fetcher) Rendered() bool { return false }

type job struct {
	ID          string `json:"id"`
	Date        string `json:"date"`
	Company     string `json:"company"`
	Position    string `json:"position"`
	Location    string `json:"location"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// Fetch reads the feed and skips every element without an id, which is how
// the leading legal notice is told apart from a job.
func (f *Fetcher) Fetch(ctx context.Context) ([]normalize.Posting, error) {
	var jobs []job
	if err := f.client.Get(ctx, f.base+"/api", &jobs); err != nil {
		return nil, fmt.Errorf("remoteok %s: %w", f.board.Slug, err)
	}
	var out []normalize.Posting
	for _, j := range jobs {
		if j.ID == "" {
			continue
		}
		p := normalize.New(f.board.Source, f.board.Slug, j.ID)
		p.Company = j.Company
		p.Title = j.Position
		p.Location = j.Location
		p.Remote = true
		p.URL = j.URL
		p.DescriptionHTML = j.Description
		p.DescriptionText = normalize.Text(j.Description)
		p.PostedAt = normalize.StampFrom(time.RFC3339, j.Date)
		out = append(out, p)
	}
	return out, nil
}
