// Package workable fetches a Workable account's public job widget.
package workable

import (
	"cmp"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/fetch/httpjson"
	"github.com/tunedev/feed/internal/normalize"
)

const Base = "https://apply.workable.com"

type Fetcher struct {
	board  boards.Board
	client *httpjson.Client
	base   string
}

func New(b boards.Board, c *httpjson.Client, base string) *Fetcher {
	return &Fetcher{board: b, client: c, base: base}
}

func (f *Fetcher) Name() string   { return f.board.Name() }
func (f *Fetcher) Rendered() bool { return false }

type response struct {
	Name string `json:"name"`
	Jobs []struct {
		Shortcode     string `json:"shortcode"`
		Title         string `json:"title"`
		URL           string `json:"url"`
		PublishedOn   string `json:"published_on"`
		Telecommuting bool   `json:"telecommuting"`
		City          string `json:"city"`
		State         string `json:"state"`
		Country       string `json:"country"`
		Description   string `json:"description"`
	} `json:"jobs"`
}

// Fetch reads the whole account in one call. details=true makes the list
// call carry every description, so no call per job is needed.
func (f *Fetcher) Fetch(ctx context.Context) ([]normalize.Posting, error) {
	var r response
	url := fmt.Sprintf("%s/api/v1/widget/accounts/%s?details=true", f.base, f.board.Slug)
	if err := f.client.Get(ctx, url, &r); err != nil {
		return nil, fmt.Errorf("workable %s: %w", f.board.Slug, err)
	}
	out := make([]normalize.Posting, 0, len(r.Jobs))
	for _, j := range r.Jobs {
		p := normalize.New(f.board.Source, f.board.Slug, j.Shortcode)
		p.Company = cmp.Or(r.Name, f.board.Company)
		p.Title = strings.TrimSpace(j.Title)
		p.Location = join(j.City, j.State, j.Country)
		p.Remote = j.Telecommuting
		p.URL = j.URL
		p.DescriptionHTML = j.Description
		p.DescriptionText = normalize.Text(j.Description)
		p.PostedAt = normalize.StampFrom(time.DateOnly, j.PublishedOn)
		out = append(out, p)
	}
	return out, nil
}

func join(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ", ")
}
