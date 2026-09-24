// Package ashby fetches an Ashby job board's public listing.
package ashby

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/fetch/httpjson"
	"github.com/tunedev/feed/internal/normalize"
)

const Base = "https://api.ashbyhq.com"

type Fetcher struct {
	board  boards.Board
	client *httpjson.Client
	base   string
}

// New serves a board with a company name, since Ashby's API omits it.
func New(b boards.Board, c *httpjson.Client, base string) (*Fetcher, error) {
	if b.Company == "" {
		return nil, fmt.Errorf("ashby %s: company is required", b.Slug)
	}
	return &Fetcher{board: b, client: c, base: base}, nil
}

func (f *Fetcher) Name() string   { return f.board.Name() }
func (f *Fetcher) Rendered() bool { return false }

type response struct {
	Jobs []struct {
		ID               string `json:"id"`
		Title            string `json:"title"`
		Location         string `json:"location"`
		IsRemote         bool   `json:"isRemote"`
		IsListed         bool   `json:"isListed"`
		PublishedAt      string `json:"publishedAt"`
		JobURL           string `json:"jobUrl"`
		DescriptionHTML  string `json:"descriptionHtml"`
		DescriptionPlain string `json:"descriptionPlain"`
	} `json:"jobs"`
}

// Fetch reads the whole board in one call; Ashby offers no filtering or
// paging. Unlisted jobs are skipped: the company chose not to show them.
func (f *Fetcher) Fetch(ctx context.Context) ([]normalize.Posting, error) {
	var r response
	url := fmt.Sprintf("%s/posting-api/job-board/%s?includeCompensation=true", f.base, f.board.Slug)
	if err := f.client.Get(ctx, url, &r); err != nil {
		return nil, fmt.Errorf("ashby %s: %w", f.board.Slug, err)
	}
	var out []normalize.Posting
	for _, j := range r.Jobs {
		if !j.IsListed {
			continue
		}
		p := normalize.New(f.board.Source, f.board.Slug, j.ID)
		p.Company = f.board.Company
		p.Title = j.Title
		p.Location = j.Location
		p.Remote = j.IsRemote
		p.URL = j.JobURL
		p.DescriptionHTML = j.DescriptionHTML
		p.DescriptionText = cmp.Or(j.DescriptionPlain, normalize.Text(j.DescriptionHTML))
		p.PostedAt = normalize.StampFrom(time.RFC3339, j.PublishedAt)
		out = append(out, p)
	}
	return out, nil
}
